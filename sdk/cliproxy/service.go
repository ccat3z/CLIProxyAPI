// Package cliproxy provides the core service implementation for the CLI Proxy API.
// It includes service lifecycle management, authentication handling, file watching,
// and integration with various AI service providers through a unified interface.
package cliproxy

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/api"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/registry"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/runtime/executor"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/runtime/limiter"
	iusage "github.com/router-for-me/CLIProxyAPI/v7/internal/usage"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/util"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/watcher"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/watcher/synthesizer"
	sdkaccess "github.com/router-for-me/CLIProxyAPI/v7/sdk/access"
	sdkAuth "github.com/router-for-me/CLIProxyAPI/v7/sdk/auth"
	coreauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/usage"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/config"
	log "github.com/sirupsen/logrus"
)

// Service wraps the proxy server lifecycle so external programs can embed the CLI proxy.
// It manages the complete lifecycle including authentication, file watching, HTTP server,
// and integration with various AI service providers.
type Service struct {
	// cfg holds the current application configuration.
	cfg *config.Config

	// cfgMu protects concurrent access to the configuration.
	cfgMu sync.RWMutex

	// configUpdateMu serializes config updates from the watcher.
	configUpdateMu sync.Mutex

	// configPath is the path to the configuration file.
	configPath string

	// tokenProvider handles loading token-based clients.
	tokenProvider TokenClientProvider

	// apiKeyProvider handles loading API key-based clients.
	apiKeyProvider APIKeyClientProvider

	// watcherFactory creates file watcher instances.
	watcherFactory WatcherFactory

	// hooks provides lifecycle callbacks.
	hooks Hooks

	// serverOptions contains additional server configuration options.
	serverOptions []api.ServerOption

	// server is the HTTP API server instance.
	server *api.Server

	// serverErr channel for server startup/shutdown errors.
	serverErr chan error

	// watcher handles file system monitoring.
	watcher *WatcherWrapper

	// watcherCancel cancels the watcher context.
	watcherCancel context.CancelFunc

	// authUpdates channel for authentication updates.
	authUpdates chan watcher.AuthUpdate

	// authQueueStop cancels the auth update queue processing.
	authQueueStop context.CancelFunc

	// authManager handles legacy authentication operations.
	authManager *sdkAuth.Manager

	// accessManager handles request authentication providers.
	accessManager *sdkaccess.Manager

	// coreManager handles core authentication and execution.
	coreManager *coreauth.Manager

	// shutdownOnce ensures shutdown is called only once.
	shutdownOnce sync.Once
}

const (
	modelRegistrationMaxWorkersPerCategory         = 5
	modelRegistrationMaxWorkersOpenAICompatibility = 20
)

const (
	modelRegistrationPhaseConfigAPIKey = iota
	modelRegistrationPhaseOther
)

type modelRegistrationTask struct {
	phase    int
	category string
	run      func(*openAICompatibilityRegistrationCache)
}

type executorRegistrationOptions struct {
	includeBaseline   bool
	forceReplaceAuths bool
	auths             []*coreauth.Auth
}

// RegisterUsagePlugin registers a usage plugin on the global usage manager.
// This allows external code to monitor API usage and token consumption.
//
// Parameters:
//   - plugin: The usage plugin to register
func (s *Service) RegisterUsagePlugin(plugin usage.Plugin) {
	usage.RegisterPlugin(plugin)
}

// reregisterExecutors rebuilds the executor set for all currently registered auths.
func (s *Service) reregisterExecutors(ctx context.Context) {
	if s == nil || s.coreManager == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	s.registerAvailableExecutors(ctx, executorRegistrationOptions{
		includeBaseline:   false,
		forceReplaceAuths: true,
		auths:             s.coreManager.List(),
	})
	s.coreManager.RefreshSchedulerAll()
}

func (s *Service) registerModelsForAuthBatch(ctx context.Context, auths []*coreauth.Auth) {
	if s == nil || s.coreManager == nil || len(auths) == 0 {
		return
	}
	tasks := make([]modelRegistrationTask, 0, len(auths))
	for _, auth := range auths {
		if auth == nil {
			continue
		}
		authForRegistration := auth.Clone()
		tasks = append(tasks, modelRegistrationTask{
			phase:    modelRegistrationPhase(authForRegistration),
			category: modelRegistrationCategory(authForRegistration),
			run: func(compatCache *openAICompatibilityRegistrationCache) {
				s.completeModelRegistrationForAuthWithCache(ctx, authForRegistration, compatCache)
			},
		})
	}
	s.runModelRegistrationTasks(ctx, tasks)
}

func (s *Service) runModelRegistrationTasks(ctx context.Context, tasks []modelRegistrationTask) {
	if len(tasks) == 0 {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	configAPIKeyTasks := make([]modelRegistrationTask, 0)
	otherTasks := make([]modelRegistrationTask, 0)
	for _, task := range tasks {
		if task.phase == modelRegistrationPhaseConfigAPIKey {
			configAPIKeyTasks = append(configAPIKeyTasks, task)
			continue
		}
		otherTasks = append(otherTasks, task)
	}

	compatCache := s.newOpenAICompatibilityRegistrationCache()
	s.runModelRegistrationTaskPhase(ctx, configAPIKeyTasks, compatCache)
	s.runModelRegistrationTaskPhase(ctx, otherTasks, compatCache)
}

func (s *Service) runModelRegistrationTaskPhase(ctx context.Context, tasks []modelRegistrationTask, compatCache *openAICompatibilityRegistrationCache) {
	if len(tasks) == 0 {
		return
	}

	grouped := make(map[string][]modelRegistrationTask)
	order := make([]string, 0)
	for _, task := range tasks {
		if task.run == nil {
			continue
		}
		category := strings.ToLower(strings.TrimSpace(task.category))
		if category == "" {
			category = "unknown"
		}
		if _, exists := grouped[category]; !exists {
			order = append(order, category)
		}
		grouped[category] = append(grouped[category], task)
	}

	var wg sync.WaitGroup
	for _, category := range order {
		group := grouped[category]
		workers := len(group)
		maxWorkers := modelRegistrationMaxWorkersForCategory(category)
		if workers > maxWorkers {
			workers = maxWorkers
		}
		if workers <= 0 {
			continue
		}

		taskCh := make(chan modelRegistrationTask)
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for task := range taskCh {
					select {
					case <-ctx.Done():
						return
					default:
					}
					task.run(compatCache)
				}
			}()
		}
		go func(group []modelRegistrationTask) {
			defer close(taskCh)
			for _, task := range group {
				select {
				case <-ctx.Done():
					return
				case taskCh <- task:
				}
			}
		}(group)
	}
	wg.Wait()
}

func modelRegistrationPhase(auth *coreauth.Auth) int {
	if coreauth.IsConfigAPIKeyAuth(auth) {
		return modelRegistrationPhaseConfigAPIKey
	}
	return modelRegistrationPhaseOther
}

func modelRegistrationCategory(auth *coreauth.Auth) string {
	if auth == nil {
		return "unknown"
	}
	provider := strings.ToLower(strings.TrimSpace(auth.Provider))
	if compatProviderKey, _, compatDetected := openAICompatInfoFromAuth(auth); compatDetected {
		if compatProviderKey != "" {
			provider = compatProviderKey
		} else {
			provider = "openai-compatibility"
		}
	}
	if provider == "" {
		provider = "unknown"
	}

	authKind := auth.AuthKind()
	if authKind == "" {
		return provider
	}
	return provider + ":" + authKind
}

func modelRegistrationMaxWorkersForCategory(category string) int {
	category = strings.ToLower(strings.TrimSpace(category))
	if strings.HasPrefix(category, "openai-compatible-") || strings.HasPrefix(category, "openai-compatibility") {
		return modelRegistrationMaxWorkersOpenAICompatibility
	}
	return modelRegistrationMaxWorkersPerCategory
}

// newDefaultAuthManager creates a default authentication manager.
// Provider-specific authenticators were removed alongside the OAuth login flows;
// the manager now coordinates persistence only via the shared token store.
func newDefaultAuthManager() *sdkAuth.Manager {
	return sdkAuth.NewManager(sdkAuth.GetTokenStore())
}

func (s *Service) ensureAuthUpdateQueue(ctx context.Context) {
	if s == nil {
		return
	}
	if s.authUpdates == nil {
		s.authUpdates = make(chan watcher.AuthUpdate, 256)
	}
	if s.authQueueStop != nil {
		return
	}
	queueCtx, cancel := context.WithCancel(ctx)
	s.authQueueStop = cancel
	go s.consumeAuthUpdates(queueCtx)
}

func (s *Service) consumeAuthUpdates(ctx context.Context) {
	ctx = coreauth.WithSkipPersist(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case update, ok := <-s.authUpdates:
			if !ok {
				return
			}
			updates := []watcher.AuthUpdate{update}
		labelDrain:
			for {
				select {
				case nextUpdate := <-s.authUpdates:
					updates = append(updates, nextUpdate)
				default:
					break labelDrain
				}
			}
			s.handleAuthUpdates(ctx, updates)
		}
	}
}

func (s *Service) emitAuthUpdate(ctx context.Context, update watcher.AuthUpdate) {
	if s == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if s.watcher != nil && s.watcher.DispatchRuntimeAuthUpdate(update) {
		return
	}
	if s.authUpdates != nil {
		select {
		case s.authUpdates <- update:
			return
		default:
			log.Debugf("auth update queue saturated, applying inline action=%v id=%s", update.Action, update.ID)
		}
	}
	s.handleAuthUpdate(ctx, update)
}

func (s *Service) handleAuthUpdate(ctx context.Context, update watcher.AuthUpdate) {
	s.handleAuthUpdates(ctx, []watcher.AuthUpdate{update})
}

func (s *Service) handleAuthUpdates(ctx context.Context, updates []watcher.AuthUpdate) {
	if s == nil {
		return
	}
	updates = coalesceAuthUpdates(updates)
	s.cfgMu.RLock()
	cfg := s.cfg
	s.cfgMu.RUnlock()
	if cfg == nil || s.coreManager == nil {
		return
	}

	registrationCtx := coreauth.WithDeferredAPIKeyModelAliasRebuild(ctx)
	tasks := make([]modelRegistrationTask, 0, len(updates))
	needsExecutorSync := false
	needsAliasRebuild := false
	for _, update := range updates {
		switch update.Action {
		case watcher.AuthUpdateActionAdd, watcher.AuthUpdateActionModify:
			if update.Auth == nil || update.Auth.ID == "" {
				continue
			}
			auth := s.prepareCoreAuthForModelRegistration(registrationCtx, update.Auth)
			if auth == nil {
				continue
			}
			needsAliasRebuild = true
			authForRegistration := auth
			tasks = append(tasks, modelRegistrationTask{
				phase:    modelRegistrationPhase(authForRegistration),
				category: modelRegistrationCategory(authForRegistration),
				run: func(compatCache *openAICompatibilityRegistrationCache) {
					s.completeModelRegistrationForAuthWithCache(registrationCtx, authForRegistration, compatCache)
				},
			})
			needsExecutorSync = true
		case watcher.AuthUpdateActionDelete:
			id := update.ID
			if id == "" && update.Auth != nil {
				id = update.Auth.ID
			}
			if id == "" {
				continue
			}
			s.applyCoreAuthRemoval(registrationCtx, id)
			needsAliasRebuild = true
		default:
			log.Debugf("received unknown auth update action: %v", update.Action)
		}
	}

	if needsAliasRebuild {
		s.coreManager.RefreshAPIKeyModelAlias()
	}
	s.runModelRegistrationTasks(registrationCtx, tasks)
	if needsExecutorSync {
		s.reregisterExecutors(registrationCtx)
	}
}

func coalesceAuthUpdates(updates []watcher.AuthUpdate) []watcher.AuthUpdate {
	if len(updates) <= 1 {
		return updates
	}
	order := make([]string, 0, len(updates))
	byID := make(map[string]watcher.AuthUpdate, len(updates))
	unkeyed := make([]watcher.AuthUpdate, 0)
	for _, update := range updates {
		id := authUpdateID(update)
		if id == "" {
			unkeyed = append(unkeyed, update)
			continue
		}
		if _, exists := byID[id]; !exists {
			order = append(order, id)
		}
		byID[id] = update
	}
	if len(byID) == 0 {
		return unkeyed
	}
	out := make([]watcher.AuthUpdate, 0, len(byID)+len(unkeyed))
	for _, id := range order {
		out = append(out, byID[id])
	}
	out = append(out, unkeyed...)
	return out
}

func authUpdateID(update watcher.AuthUpdate) string {
	if strings.TrimSpace(update.ID) != "" {
		return strings.TrimSpace(update.ID)
	}
	if update.Auth != nil {
		return strings.TrimSpace(update.Auth.ID)
	}
	return ""
}

func (s *Service) applyCoreAuthAddOrUpdate(ctx context.Context, auth *coreauth.Auth) {
	auth = s.prepareCoreAuthForModelRegistration(ctx, auth)
	if auth == nil {
		return
	}
	s.completeModelRegistrationForAuth(ctx, auth)
	s.reregisterExecutors(ctx)
}

func (s *Service) prepareCoreAuthForModelRegistration(ctx context.Context, auth *coreauth.Auth) *coreauth.Auth {
	if s == nil || s.coreManager == nil || auth == nil || auth.ID == "" {
		return nil
	}
	auth = auth.Clone()
	s.ensureExecutorsForAuth(auth)

	// IMPORTANT: Update coreManager FIRST, before model registration.
	// This ensures that configuration changes (proxy_url, prefix, etc.) take effect
	// immediately for API calls, rather than waiting for model registration to complete.
	op := "register"
	var err error
	if existing, ok := s.coreManager.GetByID(auth.ID); ok {
		auth.CreatedAt = existing.CreatedAt
		if !existing.Disabled && existing.Status != coreauth.StatusDisabled && !auth.Disabled && auth.Status != coreauth.StatusDisabled {
			auth.LastRefreshedAt = existing.LastRefreshedAt
			auth.NextRefreshAfter = existing.NextRefreshAfter
			if len(auth.ModelStates) == 0 && len(existing.ModelStates) > 0 {
				auth.ModelStates = existing.ModelStates
			}
		}
		op = "update"
		_, err = s.coreManager.Update(ctx, auth)
	} else {
		_, err = s.coreManager.Register(ctx, auth)
	}
	if err != nil {
		log.Errorf("failed to %s auth %s: %v", op, auth.ID, err)
		current, ok := s.coreManager.GetByID(auth.ID)
		if !ok || current.Disabled {
			GlobalModelRegistry().UnregisterClient(auth.ID)
			return nil
		}
		auth = current
	}
	return auth
}

func (s *Service) completeModelRegistrationForAuth(ctx context.Context, auth *coreauth.Auth) {
	s.completeModelRegistrationForAuthWithCache(ctx, auth, nil)
}

func (s *Service) completeModelRegistrationForAuthWithCache(ctx context.Context, auth *coreauth.Auth, compatCache *openAICompatibilityRegistrationCache) {
	if s == nil || s.coreManager == nil || auth == nil || auth.ID == "" {
		return
	}
	s.registerModelsForAuthWithCache(ctx, auth, compatCache)
	s.coreManager.ReconcileRegistryModelStates(ctx, auth.ID)

	// Refresh the scheduler entry so that the auth's supportedModelSet is rebuilt
	// from the now-populated global model registry. Without this, newly added auths
	// have an empty supportedModelSet (because Register/Update upserts into the
	// scheduler before registerModelsForAuth runs) and are invisible to the scheduler.
	s.coreManager.RefreshSchedulerEntry(auth.ID)
}

func (s *Service) applyCoreAuthRemoval(ctx context.Context, id string) {
	if s == nil || id == "" {
		return
	}
	if s.coreManager == nil {
		return
	}
	id = strings.TrimSpace(id)
	GlobalModelRegistry().UnregisterClient(id)
	s.coreManager.Remove(ctx, id)
	limiter.DefaultLimiter().RemoveAllForAuth(id)
	s.reregisterExecutors(ctx)
}

func (s *Service) applyRetryConfig(cfg *config.Config) {
	if s == nil || s.coreManager == nil || cfg == nil {
		return
	}
	maxInterval := time.Duration(cfg.MaxRetryInterval) * time.Second
	s.coreManager.SetRetryConfig(cfg.RequestRetry, maxInterval, cfg.MaxRetryCredentials)
	coreauth.SetTransientErrorCooldownSeconds(cfg.TransientErrorCooldownSeconds)
}

func (s *Service) configureCooldownStateStore(cfg *config.Config) {
	if s == nil || s.coreManager == nil {
		return
	}
	if cfg == nil || !cfg.SaveCooldownStatus {
		s.coreManager.SetCooldownStateStore(nil)
		return
	}
	stateDir := resolveCooldownStateDir(s.configPath)
	if stateDir == "" {
		s.coreManager.SetCooldownStateStore(nil)
		return
	}
	s.coreManager.SetCooldownStateStore(coreauth.NewFileCooldownStateStoreWithAuthDir(stateDir, stateDir))
}

// resolveCooldownStateDir returns the directory used to persist per-auth cooldown
// state files. On the custom branch there is no dedicated auth directory, so the
// directory holding the config file (the runtime data location) is used.
func resolveCooldownStateDir(configPath string) string {
	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		return ""
	}
	return filepath.Dir(configPath)
}

func openAICompatInfoFromAuth(a *coreauth.Auth) (providerKey string, compatName string, ok bool) {
	if a == nil {
		return "", "", false
	}
	if len(a.Attributes) > 0 {
		providerKey = strings.TrimSpace(a.Attributes["provider_key"])
		compatName = strings.TrimSpace(a.Attributes["compat_name"])
		if compatName != "" {
			if providerKey == "" {
				providerKey = compatName
			}
			return util.OpenAICompatibleProviderKey(providerKey), compatName, true
		}
	}
	if strings.EqualFold(strings.TrimSpace(a.Provider), "openai-compatibility") {
		compatName = strings.TrimSpace(a.Label)
		providerKey = compatName
		if providerKey == "" {
			providerKey = "openai-compatibility"
		}
		return util.OpenAICompatibleProviderKey(providerKey), compatName, true
	}
	return "", "", false
}

type openAICompatibilityRegistrationCache struct {
	byName map[string]*openAICompatibilityRegistrationEntry
}

type openAICompatibilityRegistrationEntry struct {
	providerKey string
	models      []*ModelInfo
}

func (s *Service) newOpenAICompatibilityRegistrationCache() *openAICompatibilityRegistrationCache {
	if s == nil {
		return nil
	}
	s.cfgMu.RLock()
	cfg := s.cfg
	s.cfgMu.RUnlock()
	if cfg == nil || len(cfg.OpenAICompatibility) == 0 {
		return nil
	}

	cache := &openAICompatibilityRegistrationCache{
		byName: make(map[string]*openAICompatibilityRegistrationEntry, len(cfg.OpenAICompatibility)),
	}
	for i := range cfg.OpenAICompatibility {
		compat := &cfg.OpenAICompatibility[i]
		if compat.Disabled {
			continue
		}
		compatName := strings.TrimSpace(compat.Name)
		key := strings.ToLower(compatName)
		if _, exists := cache.byName[key]; exists {
			continue
		}
		providerName := strings.ToLower(compatName)
		if providerName == "" {
			providerName = "openai-compatibility"
		}
		cache.byName[key] = &openAICompatibilityRegistrationEntry{
			providerKey: util.OpenAICompatibleProviderKey(providerName),
			models:      buildOpenAICompatibilityConfigModels(compat),
		}
	}
	if len(cache.byName) == 0 {
		return nil
	}
	return cache
}

func (c *openAICompatibilityRegistrationCache) lookup(compatName string) (*openAICompatibilityRegistrationEntry, bool) {
	if c == nil || len(c.byName) == 0 {
		return nil, false
	}
	entry, ok := c.byName[strings.ToLower(strings.TrimSpace(compatName))]
	return entry, ok
}

func (s *Service) unregisterOpenAICompatExecutor(providerKey string) {
	if s == nil || s.coreManager == nil {
		return
	}
	providerKey = strings.ToLower(strings.TrimSpace(providerKey))
	if providerKey == "" {
		return
	}
	existing, okExecutor := s.coreManager.Executor(providerKey)
	if !okExecutor || existing == nil {
		return
	}
	if _, okOpenAICompat := existing.(*executor.OpenAICompatExecutor); !okOpenAICompat {
		return
	}
	s.coreManager.UnregisterExecutor(providerKey)
}

func (s *Service) ensureExecutorsForAuth(a *coreauth.Auth) {
	s.ensureExecutorsForAuthWithMode(a, false)
}

func (s *Service) ensureExecutorsForAuthWithMode(a *coreauth.Auth, forceReplace bool) {
	if a == nil {
		return
	}
	s.registerAvailableExecutors(context.Background(), executorRegistrationOptions{
		auths:             []*coreauth.Auth{a},
		forceReplaceAuths: forceReplace,
	})
}

func (s *Service) registerAvailableExecutors(ctx context.Context, opts executorRegistrationOptions) {
	if s == nil || s.coreManager == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	// Keep all Service-owned executor registration paths here so native, Home,
	// and auth-derived executors stay in the same binding order.
	if opts.includeBaseline {
		s.registerExecutorsForAuths(baselineExecutorAuths(), true)
	}
	if len(opts.auths) > 0 {
		s.registerExecutorsForAuths(opts.auths, opts.forceReplaceAuths)
	}
}

func baselineExecutorAuths() []*coreauth.Auth {
	providers := []string{
		"claude",
		"openai-compatibility",
	}
	auths := make([]*coreauth.Auth, 0, len(providers))
	for _, provider := range providers {
		auth := &coreauth.Auth{
			ID:       provider,
			Provider: provider,
		}
		if provider == "openai-compatibility" {
			auth.Attributes = map[string]string{"compat_name": "openai-compatibility"}
		}
		auths = append(auths, auth)
	}
	return auths
}

func (s *Service) registerExecutorsForAuths(auths []*coreauth.Auth, forceReplace bool) {
	for _, auth := range auths {
		s.registerExecutorForAuth(auth, forceReplace)
	}
}

func (s *Service) registerExecutorForAuth(a *coreauth.Auth, forceReplace bool) {
	if s == nil || s.coreManager == nil || a == nil {
		return
	}
	// Skip disabled auth entries when (re)binding executors.
	// Disabled auths can linger during config reloads (e.g., removed OpenAI-compat entries)
	// and must not override active provider executors.
	if a.Disabled {
		return
	}
	if compatProviderKey, _, isCompat := openAICompatInfoFromAuth(a); isCompat {
		if compatProviderKey == "" {
			compatProviderKey = strings.ToLower(strings.TrimSpace(a.Provider))
		}
		if compatProviderKey == "" {
			compatProviderKey = "openai-compatibility"
		}
		if !forceReplace {
			if existingExecutor, hasExecutor := s.coreManager.Executor(compatProviderKey); hasExecutor {
				if _, isOpenAICompatExecutor := existingExecutor.(*executor.OpenAICompatExecutor); isOpenAICompatExecutor {
					return
				}
			}
		}
		s.coreManager.RegisterExecutor(executor.NewOpenAICompatExecutor(compatProviderKey, s.cfg))
		return
	}
	switch strings.ToLower(a.Provider) {
	case "claude":
		s.coreManager.RegisterExecutor(executor.NewClaudeExecutor(s.cfg))
	default:
		providerKey := strings.ToLower(strings.TrimSpace(a.Provider))
		if providerKey == "" {
			providerKey = "openai-compatibility"
		}
		if !forceReplace {
			if existingExecutor, hasExecutor := s.coreManager.Executor(providerKey); hasExecutor {
				if _, isOpenAICompatExecutor := existingExecutor.(*executor.OpenAICompatExecutor); isOpenAICompatExecutor {
					return
				}
			}
		}
		s.coreManager.RegisterExecutor(executor.NewOpenAICompatExecutor(providerKey, s.cfg))
	}
}

func (s *Service) registerResolvedModelsForAuth(a *coreauth.Auth, providerKey string, models []*ModelInfo) {
	if a == nil || a.ID == "" {
		return
	}
	providerKey = strings.ToLower(strings.TrimSpace(providerKey))
	if providerKey == "" {
		GlobalModelRegistry().UnregisterClient(a.ID)
		return
	}
	normalizedModels := make([]*ModelInfo, 0, len(models))
	for _, model := range models {
		if model == nil {
			continue
		}
		modelID := strings.TrimSpace(model.ID)
		if modelID == "" {
			continue
		}
		clone := *model
		clone.ID = modelID
		normalizedModels = append(normalizedModels, &clone)
	}
	if len(normalizedModels) == 0 {
		GlobalModelRegistry().UnregisterClient(a.ID)
		return
	}
	GlobalModelRegistry().RegisterClient(a.ID, providerKey, normalizedModels)
}

func (s *Service) applyConfigUpdate(newCfg *config.Config) {
	s.applyConfigUpdateWithAuthSynthesis(newCfg, true)
}

func (s *Service) applyWatcherConfigUpdate(newCfg *config.Config) {
	s.applyConfigUpdateWithAuthSynthesis(newCfg, false)
}

func (s *Service) applyConfigUpdateWithAuthSynthesis(newCfg *config.Config, synthesizeConfigAuths bool) {
	if s == nil {
		return
	}

	s.configUpdateMu.Lock()
	defer s.configUpdateMu.Unlock()

	previousStrategy := ""
	var previousSessionAffinity bool
	var previousSessionAffinityTTL string
	s.cfgMu.RLock()
	if s.cfg != nil {
		previousStrategy = strings.ToLower(strings.TrimSpace(s.cfg.Routing.Strategy))
		previousSessionAffinity = s.cfg.Routing.SessionAffinity
		previousSessionAffinityTTL = s.cfg.Routing.SessionAffinityTTL
	}
	s.cfgMu.RUnlock()

	if newCfg == nil {
		s.cfgMu.RLock()
		newCfg = s.cfg
		s.cfgMu.RUnlock()
	}
	if newCfg == nil {
		return
	}

	nextStrategy := strings.ToLower(strings.TrimSpace(newCfg.Routing.Strategy))
	normalizeStrategy := func(strategy string) string {
		switch strategy {
		case "fill-first", "fillfirst", "ff":
			return "fill-first"
		default:
			return "round-robin"
		}
	}
	previousStrategy = normalizeStrategy(previousStrategy)
	nextStrategy = normalizeStrategy(nextStrategy)

	nextSessionAffinity := newCfg.Routing.SessionAffinity
	nextSessionAffinityTTL := newCfg.Routing.SessionAffinityTTL

	selectorChanged := previousStrategy != nextStrategy ||
		previousSessionAffinity != nextSessionAffinity ||
		previousSessionAffinityTTL != nextSessionAffinityTTL

	if s.coreManager != nil && selectorChanged {
		var selector coreauth.Selector
		switch nextStrategy {
		case "fill-first":
			selector = &coreauth.FillFirstSelector{}
		default:
			selector = &coreauth.RoundRobinSelector{}
		}

		if nextSessionAffinity {
			ttl := time.Hour
			if ttlStr := strings.TrimSpace(nextSessionAffinityTTL); ttlStr != "" {
				if parsed, err := time.ParseDuration(ttlStr); err == nil && parsed > 0 {
					ttl = parsed
				}
			}
			selector = coreauth.NewSessionAffinitySelectorWithConfig(coreauth.SessionAffinityConfig{
				Fallback: selector,
				TTL:      ttl,
			})
		}

		s.coreManager.SetSelector(selector)
	}

	s.applyRetryConfig(newCfg)
	s.configureCooldownStateStore(newCfg)
	if s.server != nil {
		s.server.UpdateClients(newCfg)
	}
	s.cfgMu.Lock()
	s.cfg = newCfg
	s.cfgMu.Unlock()
	if s.coreManager != nil {
		s.coreManager.SetConfig(newCfg)
	}
	ctx := coreauth.WithSkipPersist(context.Background())
	var auths []*coreauth.Auth
	if s.coreManager != nil {
		auths = s.coreManager.List()
	}
	s.registerAvailableExecutors(context.Background(), executorRegistrationOptions{
		includeBaseline:   false,
		forceReplaceAuths: true,
		auths:             auths,
	})
	if synthesizeConfigAuths {
		s.registerConfigAPIKeyAuths(ctx, newCfg)
	}
	if s.coreManager != nil && newCfg.SaveCooldownStatus {
		if errRestoreCooldown := s.coreManager.RestoreCooldownStates(context.Background()); errRestoreCooldown != nil {
			log.Warnf("failed to restore cooldown state after config update: %v", errRestoreCooldown)
		}
	}
}

func (s *Service) reloadConfigFromWatcher() bool {
	if s == nil || s.watcher == nil {
		return false
	}
	return s.watcher.ReloadConfigIfChanged()
}

func (s *Service) registerConfigAPIKeyAuths(ctx context.Context, cfg *config.Config) {
	if s == nil || s.coreManager == nil || cfg == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	configSynth := synthesizer.NewConfigSynthesizer()
	auths, errSynthesize := configSynth.Synthesize(&synthesizer.SynthesisContext{
		Config:      cfg,
		Now:         time.Now(),
		IDGenerator: synthesizer.NewStableIDGenerator(),
	})
	if errSynthesize != nil {
		log.Warnf("failed to synthesize config API key auths: %v", errSynthesize)
		return
	}

	registrationCtx := coreauth.WithDeferredAPIKeyModelAliasRebuild(ctx)
	tasks := make([]modelRegistrationTask, 0, len(auths))
	needsAliasRebuild := false
	for _, auth := range auths {
		if !coreauth.IsConfigAPIKeyAuth(auth) {
			continue
		}
		prepared := s.prepareCoreAuthForModelRegistration(registrationCtx, auth)
		if prepared == nil {
			continue
		}
		needsAliasRebuild = true
		authForRegistration := prepared
		tasks = append(tasks, modelRegistrationTask{
			phase:    modelRegistrationPhaseConfigAPIKey,
			category: modelRegistrationCategory(authForRegistration),
			run: func(compatCache *openAICompatibilityRegistrationCache) {
				s.completeModelRegistrationForAuthWithCache(registrationCtx, authForRegistration, compatCache)
			},
		})
	}
	if needsAliasRebuild {
		s.coreManager.RefreshAPIKeyModelAlias()
	}
	s.runModelRegistrationTasks(registrationCtx, tasks)
}

// Run starts the service and blocks until the context is cancelled or the server stops.
// It initializes all components including authentication, file watching, HTTP server,
// and starts processing requests. The method blocks until the context is cancelled.
//
// Parameters:
//   - ctx: The context for controlling the service lifecycle
//
// Returns:
//   - error: An error if the service fails to start or run
func (s *Service) Run(ctx context.Context) error {
	if s == nil {
		return fmt.Errorf("cliproxy: service is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	usage.StartDefault(ctx)

	if s.cfg.UsageDB != "" {
		if err := iusage.InitPersistStore(s.cfg.UsageDB); err != nil {
			log.Warnf("failed to init usage persist store: %v", err)
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	defer func() {
		if err := s.Shutdown(shutdownCtx); err != nil {
			log.Errorf("service shutdown returned error: %v", err)
		}
	}()

	s.applyRetryConfig(s.cfg)
	s.configureCooldownStateStore(s.cfg)

	if s.coreManager != nil {
		if errLoad := s.coreManager.Load(ctx); errLoad != nil {
			log.Warnf("failed to load auth store: %v", errLoad)
		}
		s.registerConfigAPIKeyAuths(coreauth.WithSkipPersist(ctx), s.cfg)
		if s.cfg.SaveCooldownStatus {
			if errRestoreCooldown := s.coreManager.RestoreCooldownStates(ctx); errRestoreCooldown != nil {
				log.Warnf("failed to restore cooldown state: %v", errRestoreCooldown)
			}
		}
	}

	tokenResult, err := s.tokenProvider.Load(ctx, s.cfg)
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	if tokenResult == nil {
		tokenResult = &TokenClientResult{}
	}

	apiKeyResult, err := s.apiKeyProvider.Load(ctx, s.cfg)
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	if apiKeyResult == nil {
		apiKeyResult = &APIKeyClientResult{}
	}

	// handlers no longer depend on legacy clients; pass nil slice initially
	s.server = api.NewServer(s.cfg, s.coreManager, s.accessManager, s.configPath, s.serverOptions...)

	if s.authManager == nil {
		s.authManager = newDefaultAuthManager()
	}

	if s.hooks.OnBeforeStart != nil {
		s.hooks.OnBeforeStart(s.cfg)
	}

	s.serverErr = make(chan error, 1)
	go func() {
		if errStart := s.server.Start(); errStart != nil {
			s.serverErr <- errStart
		} else {
			s.serverErr <- nil
		}
	}()

	time.Sleep(100 * time.Millisecond)
	fmt.Printf("API server started successfully on: %s:%d\n", s.cfg.Host, s.cfg.Port)

	if s.hooks.OnAfterStart != nil {
		s.hooks.OnAfterStart(s)
	}

	var watcherWrapper *WatcherWrapper
	reloadCallback := func(newCfg *config.Config) { s.applyWatcherConfigUpdate(newCfg) }

	watcherWrapper, errCreate := s.watcherFactory(s.configPath, reloadCallback)
	if errCreate != nil {
		return fmt.Errorf("cliproxy: failed to create watcher: %w", errCreate)
	}
	s.watcher = watcherWrapper
	s.ensureAuthUpdateQueue(ctx)
	if s.authUpdates != nil {
		watcherWrapper.SetAuthUpdateQueue(s.authUpdates)
	}
	watcherWrapper.SetConfig(s.cfg)

	watcherCtx, watcherCancel := context.WithCancel(context.Background())
	s.watcherCancel = watcherCancel
	if errStart := watcherWrapper.Start(watcherCtx); errStart != nil {
		return fmt.Errorf("cliproxy: failed to start watcher: %w", errStart)
	}
	log.Info("file watcher started for config and auth directory changes")
	s.reregisterExecutors(ctx)

	// Prefer core auth manager auto refresh if available.
	if s.coreManager != nil {
		interval := 15 * time.Minute
		s.coreManager.StartAutoRefresh(context.Background(), interval)
		log.Infof("core auth auto-refresh started (interval=%s)", interval)
	}

	select {
	case <-ctx.Done():
		log.Debug("service context cancelled, shutting down...")
		return ctx.Err()
	case errServer := <-s.serverErr:
		return errServer
	}
}

// Shutdown gracefully stops background workers and the HTTP server.
// It ensures all resources are properly cleaned up and connections are closed.
// The shutdown is idempotent and can be called multiple times safely.
//
// Parameters:
//   - ctx: The context for controlling the shutdown timeout
//
// Returns:
//   - error: An error if shutdown fails
func (s *Service) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}
	var shutdownErr error
	s.shutdownOnce.Do(func() {
		if ctx == nil {
			ctx = context.Background()
		}

		// legacy refresh loop removed; only stopping core auth manager below

		if s.watcherCancel != nil {
			s.watcherCancel()
		}
		if s.coreManager != nil {
			s.coreManager.StopAutoRefresh()
		}
		if s.watcher != nil {
			if err := s.watcher.Stop(); err != nil {
				log.Errorf("failed to stop file watcher: %v", err)
				shutdownErr = err
			}
		}
		if s.authQueueStop != nil {
			s.authQueueStop()
			s.authQueueStop = nil
		}

		// no legacy clients to persist

		if s.server != nil {
			shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			if err := s.server.Stop(shutdownCtx); err != nil {
				log.Errorf("error stopping API server: %v", err)
				if shutdownErr == nil {
					shutdownErr = err
				}
			}
		}

		iusage.ClosePersistStore()

		usage.StopDefault()
	})
	return shutdownErr
}

// registerModelsForAuth (re)binds provider models in the global registry using the core auth ID as client identifier.
func (s *Service) registerModelsForAuth(ctx context.Context, a *coreauth.Auth) {
	s.registerModelsForAuthWithCache(ctx, a, nil)
}

func (s *Service) registerModelsForAuthWithCache(ctx context.Context, a *coreauth.Auth, compatCache *openAICompatibilityRegistrationCache) {
	if a == nil || a.ID == "" {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if a.Disabled {
		GlobalModelRegistry().UnregisterClient(a.ID)
		return
	}
	authKind := a.AuthKind()
	// Unregister legacy client ID (if present) to avoid double counting
	if a.Runtime != nil {
		if idGetter, ok := a.Runtime.(interface{ GetClientID() string }); ok {
			if rid := idGetter.GetClientID(); rid != "" && rid != a.ID {
				GlobalModelRegistry().UnregisterClient(rid)
			}
		}
	}
	provider := strings.ToLower(strings.TrimSpace(a.Provider))
	compatProviderKey, compatDisplayName, compatDetected := openAICompatInfoFromAuth(a)
	if compatDetected {
		provider = "openai-compatibility"
	}
	var excluded []string
	// The synthesizer pre-merges per-account and global exclusions into the "excluded_models" attribute.
	// If this attribute is present, it represents the complete list of exclusions and overrides the global config.
	if a.Attributes != nil {
		if val, ok := a.Attributes["excluded_models"]; ok && strings.TrimSpace(val) != "" {
			excluded = strings.Split(val, ",")
		}
	}
	var models []*ModelInfo
	switch provider {
	case "claude":
		models = registry.GetClaudeModels()
		if entry := s.resolveConfigClaudeKey(a); entry != nil {
			if len(entry.Models) > 0 {
				models = buildClaudeConfigModels(entry)
			}
			if authKind == "apikey" {
				excluded = entry.ExcludedModels
			}
		}
		models = applyExcludedModels(models, excluded)
	default:
		// Handle OpenAI-compatibility providers by name using config
		if s.cfg != nil {
			providerKey := provider
			compatName := strings.TrimSpace(a.Provider)
			isCompatAuth := false
			if compatDetected {
				if compatProviderKey != "" {
					providerKey = compatProviderKey
				}
				if compatDisplayName != "" {
					compatName = compatDisplayName
				}
				isCompatAuth = true
			}
			if strings.EqualFold(providerKey, "openai-compatibility") {
				isCompatAuth = true
				if a.Attributes != nil {
					if v := strings.TrimSpace(a.Attributes["compat_name"]); v != "" {
						compatName = v
					}
					if v := strings.TrimSpace(a.Attributes["provider_key"]); v != "" {
						providerKey = strings.ToLower(v)
						isCompatAuth = true
					}
				}
				if providerKey == "openai-compatibility" && compatName != "" {
					providerKey = strings.ToLower(compatName)
				}
			} else if a.Attributes != nil {
				if v := strings.TrimSpace(a.Attributes["compat_name"]); v != "" {
					compatName = v
					isCompatAuth = true
				}
				if v := strings.TrimSpace(a.Attributes["provider_key"]); v != "" {
					providerKey = strings.ToLower(v)
					isCompatAuth = true
				}
			}
			if cached, ok := compatCache.lookup(compatName); ok {
				isCompatAuth = true
				if providerKey == "" {
					providerKey = cached.providerKey
				}
				if providerKey == "" {
					providerKey = "openai-compatibility"
				}
				ms := cached.models
				if len(ms) > 0 {
					s.registerResolvedModelsForAuth(a, providerKey, applyModelPrefixes(ms, a.Prefix, s.cfg.ForceModelPrefix))
				} else {
					GlobalModelRegistry().UnregisterClient(a.ID)
				}
				return
			}
			for i := range s.cfg.OpenAICompatibility {
				compat := &s.cfg.OpenAICompatibility[i]
				if compat.Disabled {
					continue
				}
				if strings.EqualFold(compat.Name, compatName) {
					isCompatAuth = true
					ms := buildOpenAICompatibilityConfigModels(compat)
					// Register and return
					if len(ms) > 0 {
						if providerKey == "" {
							providerKey = "openai-compatibility"
						}
						s.registerResolvedModelsForAuth(a, providerKey, applyModelPrefixes(ms, a.Prefix, s.cfg.ForceModelPrefix))
					} else {
						GlobalModelRegistry().UnregisterClient(a.ID)
					}
					return
				}
			}
			if isCompatAuth {
				if len(models) > 0 {
					s.registerResolvedModelsForAuth(a, providerKey, applyModelPrefixes(models, a.Prefix, s.cfg != nil && s.cfg.ForceModelPrefix))
				} else {
					// No matching provider found or models removed entirely; drop any prior registration.
					GlobalModelRegistry().UnregisterClient(a.ID)
				}
				return
			}
		}
	}
	key := provider
	if key == "" {
		key = strings.ToLower(strings.TrimSpace(a.Provider))
	}
	if len(models) > 0 {
		s.registerResolvedModelsForAuth(a, key, applyModelPrefixes(models, a.Prefix, s.cfg != nil && s.cfg.ForceModelPrefix))
		return
	}

	GlobalModelRegistry().UnregisterClient(a.ID)
}

func (s *Service) resolveConfigClaudeKey(auth *coreauth.Auth) *config.ClaudeKey {
	if auth == nil || s.cfg == nil {
		return nil
	}
	var attrKey, attrBase string
	if auth.Attributes != nil {
		attrKey = strings.TrimSpace(auth.Attributes["api_key"])
		attrBase = strings.TrimSpace(auth.Attributes["base_url"])
	}
	for i := range s.cfg.ClaudeKey {
		entry := &s.cfg.ClaudeKey[i]
		cfgKey := strings.TrimSpace(entry.APIKey)
		cfgBase := strings.TrimSpace(entry.BaseURL)
		if attrKey != "" && attrBase != "" {
			if strings.EqualFold(cfgKey, attrKey) && strings.EqualFold(cfgBase, attrBase) {
				return entry
			}
			continue
		}
		if attrKey != "" && strings.EqualFold(cfgKey, attrKey) {
			if cfgBase == "" || strings.EqualFold(cfgBase, attrBase) {
				return entry
			}
		}
		if attrKey == "" && attrBase != "" && strings.EqualFold(cfgBase, attrBase) {
			return entry
		}
	}
	if attrKey != "" {
		for i := range s.cfg.ClaudeKey {
			entry := &s.cfg.ClaudeKey[i]
			if strings.EqualFold(strings.TrimSpace(entry.APIKey), attrKey) {
				return entry
			}
		}
	}
	return nil
}

func applyExcludedModels(models []*ModelInfo, excluded []string) []*ModelInfo {
	if len(models) == 0 || len(excluded) == 0 {
		return models
	}

	patterns := make([]string, 0, len(excluded))
	for _, item := range excluded {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			patterns = append(patterns, strings.ToLower(trimmed))
		}
	}
	if len(patterns) == 0 {
		return models
	}

	filtered := make([]*ModelInfo, 0, len(models))
	for _, model := range models {
		if model == nil {
			continue
		}
		modelID := strings.ToLower(strings.TrimSpace(model.ID))
		blocked := false
		for _, pattern := range patterns {
			if matchWildcard(pattern, modelID) {
				blocked = true
				break
			}
		}
		if !blocked {
			filtered = append(filtered, model)
		}
	}
	return filtered
}

func applyModelPrefixes(models []*ModelInfo, prefix string, forceModelPrefix bool) []*ModelInfo {
	trimmedPrefix := strings.TrimSpace(prefix)
	if trimmedPrefix == "" || len(models) == 0 {
		return models
	}

	out := make([]*ModelInfo, 0, len(models)*2)
	seen := make(map[string]struct{}, len(models)*2)

	addModel := func(model *ModelInfo) {
		if model == nil {
			return
		}
		id := strings.TrimSpace(model.ID)
		if id == "" {
			return
		}
		if _, exists := seen[id]; exists {
			return
		}
		seen[id] = struct{}{}
		out = append(out, model)
	}

	for _, model := range models {
		if model == nil {
			continue
		}
		baseID := strings.TrimSpace(model.ID)
		if baseID == "" {
			continue
		}
		if !forceModelPrefix || trimmedPrefix == baseID {
			addModel(model)
		}
		clone := *model
		clone.ID = trimmedPrefix + "/" + baseID
		addModel(&clone)
	}
	return out
}

// matchWildcard performs case-insensitive wildcard matching where '*' matches any substring.
func matchWildcard(pattern, value string) bool {
	if pattern == "" {
		return false
	}

	// Fast path for exact match (no wildcard present).
	if !strings.Contains(pattern, "*") {
		return pattern == value
	}

	parts := strings.Split(pattern, "*")
	// Handle prefix.
	if prefix := parts[0]; prefix != "" {
		if !strings.HasPrefix(value, prefix) {
			return false
		}
		value = value[len(prefix):]
	}

	// Handle suffix.
	if suffix := parts[len(parts)-1]; suffix != "" {
		if !strings.HasSuffix(value, suffix) {
			return false
		}
		value = value[:len(value)-len(suffix)]
	}

	// Handle middle segments in order.
	for i := 1; i < len(parts)-1; i++ {
		segment := parts[i]
		if segment == "" {
			continue
		}
		idx := strings.Index(value, segment)
		if idx < 0 {
			return false
		}
		value = value[idx+len(segment):]
	}

	return true
}

type modelEntry interface {
	GetName() string
	GetAlias() string
	GetDisplayName() string
	GetExtra() map[string]any
}

func buildOpenAICompatibilityConfigModels(compat *config.OpenAICompatibility) []*ModelInfo {
	if compat == nil || len(compat.Models) == 0 {
		return nil
	}
	now := time.Now().Unix()
	models := make([]*ModelInfo, 0, len(compat.Models))
	for i := range compat.Models {
		model := compat.Models[i]
		modelID := strings.TrimSpace(model.Alias)
		if modelID == "" {
			modelID = strings.TrimSpace(model.Name)
		}
		if modelID == "" {
			continue
		}
		modelType := "openai-compatibility"
		if model.Image {
			modelType = registry.OpenAIImageModelType
		}
		thinking := model.Thinking
		if thinking == nil && !model.Image {
			thinking = &registry.ThinkingSupport{Levels: []string{"low", "medium", "high"}}
		}
		inputModalities := normalizeCompatConfigModalities(model.InputModalities)
		outputModalities := normalizeCompatConfigModalities(model.OutputModalities)
		displayName := strings.TrimSpace(model.DisplayName)
		if displayName == "" {
			displayName = modelID
		}
		models = append(models, &ModelInfo{
			ID:                        modelID,
			Object:                    "model",
			Created:                   now,
			OwnedBy:                   compat.Name,
			Type:                      modelType,
			DisplayName:               displayName,
			UserDefined:               false,
			Thinking:                  thinking,
			Extra:                     model.Extra,
			SupportedInputModalities:  inputModalities,
			SupportedOutputModalities: outputModalities,
		})
	}
	return models
}

func normalizeCompatConfigModalities(raw []string) []string {
	if len(raw) == 0 {
		return nil
	}
	out := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, item := range raw {
		modality := strings.ToLower(strings.TrimSpace(item))
		if modality == "" {
			continue
		}
		if _, exists := seen[modality]; exists {
			continue
		}
		seen[modality] = struct{}{}
		out = append(out, modality)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func buildConfigModels[T modelEntry](models []T, ownedBy, modelType string) []*ModelInfo {
	if len(models) == 0 {
		return nil
	}
	now := time.Now().Unix()
	out := make([]*ModelInfo, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for i := range models {
		model := models[i]
		name := strings.TrimSpace(model.GetName())
		alias := strings.TrimSpace(model.GetAlias())
		if alias == "" {
			alias = name
		}
		if alias == "" {
			continue
		}
		key := strings.ToLower(alias)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		display := strings.TrimSpace(model.GetDisplayName())
		if display == "" {
			display = name
		}
		if display == "" {
			display = alias
		}
		info := &ModelInfo{
			ID:          alias,
			Object:      "model",
			Created:     now,
			OwnedBy:     ownedBy,
			Type:        modelType,
			DisplayName: display,
			UserDefined: true,
		}
		if extra := model.GetExtra(); len(extra) > 0 {
			info.Extra = extra
		}
		if name != "" {
			if upstream := registry.LookupStaticModelInfo(name); upstream != nil && upstream.Thinking != nil {
				info.Thinking = upstream.Thinking
			}
		}
		out = append(out, info)
	}
	return out
}

func buildClaudeConfigModels(entry *config.ClaudeKey) []*ModelInfo {
	if entry == nil {
		return nil
	}
	return buildConfigModels(entry.Models, "anthropic", "claude")
}

func rewriteModelInfoName(name, oldID, newID string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return name
	}
	oldID = strings.TrimSpace(oldID)
	newID = strings.TrimSpace(newID)
	if oldID == "" || newID == "" {
		return name
	}
	if strings.EqualFold(oldID, newID) {
		return name
	}
	if strings.EqualFold(trimmed, oldID) {
		return newID
	}
	if strings.HasSuffix(trimmed, "/"+oldID) {
		prefix := strings.TrimSuffix(trimmed, oldID)
		return prefix + newID
	}
	if trimmed == "models/"+oldID {
		return "models/" + newID
	}
	return name
}
