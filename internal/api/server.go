// Package api provides the HTTP API server implementation for the CLI Proxy API.
// It includes the main server struct, routing setup, middleware for CORS and authentication,
// and integration with various AI API handlers (OpenAI, Claude).
// The server supports hot-reloading of clients and configuration.
package api

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/access"
	managementHandlers "github.com/router-for-me/CLIProxyAPI/v7/internal/api/handlers/management"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/api/middleware"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/logging"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/managementasset"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/util"
	sdkaccess "github.com/router-for-me/CLIProxyAPI/v7/sdk/access"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/api/handlers"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/api/handlers/claude"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/api/handlers/openai"
	sdkAuth "github.com/router-for-me/CLIProxyAPI/v7/sdk/auth"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

var corsExposedResponseHeaders = []string{
	"X-CPA-VERSION",
	"X-CPA-COMMIT",
	"X-CPA-BUILD-DATE",
	"X-CPA-HOME-VERSION",
	"X-CPA-HOME-BUILD-DATE",
	"X-SERVER-VERSION",
	"X-SERVER-BUILD-DATE",
}

var corsExposedResponseHeadersJoined = strings.Join(corsExposedResponseHeaders, ", ")

type serverOptionConfig struct {
	extraMiddleware      []gin.HandlerFunc
	engineConfigurator   func(*gin.Engine)
	routerConfigurator   func(*gin.Engine, *handlers.BaseAPIHandler, *config.Config)
	requestLoggerFactory func(*config.Config, string) logging.RequestLogger
	localPassword        string
	keepAliveEnabled     bool
	keepAliveTimeout     time.Duration
	keepAliveOnTimeout   func()
	postAuthHook         auth.PostAuthHook
	postAuthPersistHook  auth.PostAuthHook
	configReloadHook     func(context.Context, *config.Config)
}

// ServerOption customises HTTP server construction.
type ServerOption func(*serverOptionConfig)

func defaultRequestLoggerFactory(cfg *config.Config, configPath string) logging.RequestLogger {
	configDir := filepath.Dir(configPath)
	logsDir := logging.ResolveLogDirectory(cfg)
	return logging.NewFileRequestLogger(cfg.RequestLog, logsDir, configDir, cfg.ErrorLogsMaxFiles)
}

func effectiveSDKConfig(cfg *config.Config) *config.SDKConfig {
	if cfg == nil {
		return nil
	}
	sdkCfg := cfg.SDKConfig
	if cfg.CommercialMode {
		sdkCfg.RequestLog = false
	}
	return &sdkCfg
}

// WithMiddleware appends additional Gin middleware during server construction.
func WithMiddleware(mw ...gin.HandlerFunc) ServerOption {
	return func(cfg *serverOptionConfig) {
		cfg.extraMiddleware = append(cfg.extraMiddleware, mw...)
	}
}

// WithEngineConfigurator allows callers to mutate the Gin engine prior to middleware setup.
func WithEngineConfigurator(fn func(*gin.Engine)) ServerOption {
	return func(cfg *serverOptionConfig) {
		cfg.engineConfigurator = fn
	}
}

// WithRouterConfigurator appends a callback after default routes are registered.
func WithRouterConfigurator(fn func(*gin.Engine, *handlers.BaseAPIHandler, *config.Config)) ServerOption {
	return func(cfg *serverOptionConfig) {
		cfg.routerConfigurator = fn
	}
}

// WithLocalManagementPassword stores a runtime-only management password accepted for localhost requests.
func WithLocalManagementPassword(password string) ServerOption {
	return func(cfg *serverOptionConfig) {
		cfg.localPassword = password
	}
}

// WithKeepAliveEndpoint enables a keep-alive endpoint with the provided timeout and callback.
func WithKeepAliveEndpoint(timeout time.Duration, onTimeout func()) ServerOption {
	return func(cfg *serverOptionConfig) {
		if timeout <= 0 || onTimeout == nil {
			return
		}
		cfg.keepAliveEnabled = true
		cfg.keepAliveTimeout = timeout
		cfg.keepAliveOnTimeout = onTimeout
	}
}

// WithRequestLoggerFactory customises request logger creation.
func WithRequestLoggerFactory(factory func(*config.Config, string) logging.RequestLogger) ServerOption {
	return func(cfg *serverOptionConfig) {
		cfg.requestLoggerFactory = factory
	}
}

// WithPostAuthHook registers a hook to be called after auth record creation.
func WithPostAuthHook(hook auth.PostAuthHook) ServerOption {
	return func(cfg *serverOptionConfig) {
		cfg.postAuthHook = hook
	}
}

// WithPostAuthPersistHook registers a hook to be called after auth persistence.
func WithPostAuthPersistHook(hook auth.PostAuthHook) ServerOption {
	return func(cfg *serverOptionConfig) {
		cfg.postAuthPersistHook = hook
	}
}

// WithConfigReloadHook registers a callback used after management saves config changes.
func WithConfigReloadHook(hook func(context.Context, *config.Config)) ServerOption {
	return func(cfg *serverOptionConfig) {
		cfg.configReloadHook = hook
	}
}

// Server represents the main API server.
// It encapsulates the Gin engine, HTTP server, handlers, and configuration.
type Server struct {
	// engine is the Gin web framework engine instance.
	engine *gin.Engine

	// server is the underlying HTTP server.
	server *http.Server

	// muxBaseListener is the shared TCP listener used to serve HTTP traffic.
	muxBaseListener net.Listener

	// muxHTTPListener receives HTTP connections selected by the multiplexer.
	muxHTTPListener *muxListener

	// handlers contains the API handlers for processing requests.
	handlers *handlers.BaseAPIHandler

	// cfg holds the current server configuration.
	cfg *config.Config

	// oldConfigYaml stores a YAML snapshot of the previous configuration for change detection.
	// This prevents issues when the config object is modified in place by Management API.
	oldConfigYaml []byte

	// accessManager handles request authentication providers.
	accessManager *sdkaccess.Manager

	// requestLogger is the request logger instance for dynamic configuration updates.
	requestLogger logging.RequestLogger
	loggerToggle  func(bool)

	// configFilePath is the absolute path to the YAML config file for persistence.
	configFilePath string

	// currentPath is the absolute path to the current working directory.
	currentPath string

	// management handler
	mgmt *managementHandlers.Handler

	// managementRoutesRegistered tracks whether the management routes have been attached to the engine.
	managementRoutesRegistered atomic.Bool
	// managementRoutesEnabled controls whether management endpoints serve real handlers.
	managementRoutesEnabled atomic.Bool

	// envManagementSecret indicates whether MANAGEMENT_PASSWORD is configured.
	envManagementSecret bool

	localPassword string

	keepAliveEnabled   bool
	keepAliveTimeout   time.Duration
	keepAliveOnTimeout func()
	keepAliveHeartbeat chan struct{}
	keepAliveStop      chan struct{}
}

// NewServer creates and initializes a new API server instance.
// It sets up the Gin engine, middleware, routes, and handlers.
//
// Parameters:
//   - cfg: The server configuration
//   - authManager: core runtime auth manager
//   - accessManager: request authentication manager
//
// Returns:
//   - *Server: A new server instance
func NewServer(cfg *config.Config, authManager *auth.Manager, accessManager *sdkaccess.Manager, configFilePath string, opts ...ServerOption) *Server {
	optionState := &serverOptionConfig{
		requestLoggerFactory: defaultRequestLoggerFactory,
	}
	for i := range opts {
		opts[i](optionState)
	}
	// Set gin mode
	if !cfg.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create gin engine
	engine := gin.New()
	if optionState.engineConfigurator != nil {
		optionState.engineConfigurator(engine)
	}

	// Add middleware
	engine.Use(logging.GinLogrusLogger())
	engine.Use(logging.GinLogrusRecovery())
	for _, mw := range optionState.extraMiddleware {
		engine.Use(mw)
	}

	// Add request logging middleware (positioned after recovery, before auth)
	// Resolve logs directory relative to the configuration file directory.
	var requestLogger logging.RequestLogger
	var toggle func(bool)
	if !cfg.CommercialMode {
		if optionState.requestLoggerFactory != nil {
			requestLogger = optionState.requestLoggerFactory(cfg, configFilePath)
		}
		if requestLogger != nil {
			engine.Use(middleware.RequestLoggingMiddleware(requestLogger))
			if setter, ok := requestLogger.(interface{ SetEnabled(bool) }); ok {
				toggle = setter.SetEnabled
			}
		}
	}

	engine.Use(corsMiddleware())
	wd, err := os.Getwd()
	if err != nil {
		wd = configFilePath
	}

	envAdminPassword, envAdminPasswordSet := os.LookupEnv("MANAGEMENT_PASSWORD")
	envAdminPassword = strings.TrimSpace(envAdminPassword)
	envManagementSecret := envAdminPasswordSet && envAdminPassword != ""

	// Create server instance
	s := &Server{
		engine:              engine,
		handlers:            handlers.NewBaseAPIHandlers(effectiveSDKConfig(cfg), authManager),
		cfg:                 cfg,
		accessManager:       accessManager,
		requestLogger:       requestLogger,
		loggerToggle:        toggle,
		configFilePath:      configFilePath,
		currentPath:         wd,
		envManagementSecret: envManagementSecret,
	}
	// Save initial YAML snapshot
	s.oldConfigYaml, _ = yaml.Marshal(cfg)
	s.applyAccessConfig(nil, cfg)
	if authManager != nil {
		authManager.SetRetryConfig(cfg.RequestRetry, time.Duration(cfg.MaxRetryInterval)*time.Second, cfg.MaxRetryCredentials)
	}
	managementasset.SetCurrentConfig(cfg)
	auth.SetQuotaCooldownDisabled(cfg.DisableCooling)
	// Initialize management handler
	s.mgmt = managementHandlers.NewHandler(cfg, configFilePath, authManager)
	s.mgmt.SetConfigReloadHook(optionState.configReloadHook)
	if optionState.localPassword != "" {
		s.mgmt.SetLocalPassword(optionState.localPassword)
	}
	logDir := logging.ResolveLogDirectory(cfg)
	if !filepath.IsAbs(logDir) {
		if configDir := filepath.Dir(configFilePath); configDir != "" {
			logDir = filepath.Join(configDir, logDir)
		}
	}
	s.mgmt.SetLogDirectory(logDir)
	if optionState.postAuthHook != nil {
		s.mgmt.SetPostAuthHook(optionState.postAuthHook)
	}
	if optionState.postAuthPersistHook != nil {
		s.mgmt.SetPostAuthPersistHook(optionState.postAuthPersistHook)
	}
	s.localPassword = optionState.localPassword

	// Setup routes
	s.setupRoutes()

	// Apply additional router configurators from options
	if optionState.routerConfigurator != nil {
		optionState.routerConfigurator(engine, s.handlers, cfg)
	}

	// Register management routes when configuration or environment secrets are available,
	// or when a local management password is provided (e.g. TUI mode).
	hasManagementSecret := cfg.RemoteManagement.SecretKey != "" || envManagementSecret || s.localPassword != ""
	s.managementRoutesEnabled.Store(hasManagementSecret)
	if hasManagementSecret {
		s.registerManagementRoutes()
	}
	engine.NoRoute(func(c *gin.Context) {
		c.AbortWithStatus(http.StatusNotFound)
	})

	if optionState.keepAliveEnabled {
		s.enableKeepAlive(optionState.keepAliveTimeout, optionState.keepAliveOnTimeout)
	}

	// Create HTTP server
	s.server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler: engine,
	}

	return s
}

// setupRoutes configures the API routes for the server.
// It defines the endpoints and associates them with their respective handlers.
func (s *Server) setupRoutes() {
	healthzHandler := func(c *gin.Context) {
		if c.Request.Method == http.MethodHead {
			c.Status(http.StatusOK)
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
	s.engine.GET("/healthz", healthzHandler)
	s.engine.HEAD("/healthz", healthzHandler)

	s.engine.GET("/management.html", s.serveManagementControlPanel)
	openaiHandlers := openai.NewOpenAIAPIHandler(s.handlers)
	claudeCodeHandlers := claude.NewClaudeCodeAPIHandler(s.handlers)
	openaiResponsesHandlers := openai.NewOpenAIResponsesAPIHandler(s.handlers)

	// OpenAI compatible API routes
	v1 := s.engine.Group("/v1")
	v1.Use(AuthMiddleware(s.accessManager))
	{
		v1.GET("/models", s.unifiedModelsHandler(openaiHandlers, claudeCodeHandlers))
		v1.POST("/chat/completions", openaiHandlers.ChatCompletions)
		v1.POST("/completions", openaiHandlers.Completions)
		v1.POST("/images/generations", openaiHandlers.ImagesGenerations)
		v1.POST("/images/edits", openaiHandlers.ImagesEdits)
		v1.POST("/messages", claudeCodeHandlers.ClaudeMessages)
		v1.POST("/messages/count_tokens", claudeCodeHandlers.ClaudeCountTokens)
		v1.GET("/responses", openaiResponsesHandlers.ResponsesWebsocket)
		v1.POST("/responses", openaiResponsesHandlers.Responses)
		v1.POST("/responses/compact", openaiResponsesHandlers.Compact)
	}

	// Codex CLI direct route aliases (chatgpt_base_url compatible)
	codexDirect := s.engine.Group("/backend-api/codex")
	codexDirect.Use(AuthMiddleware(s.accessManager))
	{
		codexDirect.GET("/responses", openaiResponsesHandlers.ResponsesWebsocket)
		codexDirect.POST("/responses", openaiResponsesHandlers.Responses)
		codexDirect.POST("/responses/compact", openaiResponsesHandlers.Compact)
	}

	// Root endpoint
	s.engine.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "CLI Proxy API Server",
			"endpoints": []string{
				"POST /v1/chat/completions",
				"POST /v1/completions",
				"GET /v1/models",
			},
		})
	})
	// Management routes are registered lazily by registerManagementRoutes when a secret is configured.
}

func (s *Server) registerManagementRoutes() {
	if s == nil || s.engine == nil || s.mgmt == nil {
		return
	}
	if !s.managementRoutesRegistered.CompareAndSwap(false, true) {
		return
	}

	log.Info("management routes registered after secret key configuration")

	mgmt := s.engine.Group("/v0/management")
	mgmt.Use(s.managementAvailabilityMiddleware(), s.mgmt.Middleware(), s.mgmt.ConfigWriteBlockMiddleware())
	{
		mgmt.GET("/usage", s.mgmt.GetUsageStatistics)
		mgmt.GET("/config", s.mgmt.GetConfig)
		mgmt.GET("/config.yaml", s.mgmt.GetConfigYAML)
		mgmt.PUT("/config.yaml", s.mgmt.PutConfigYAML)
		mgmt.GET("/latest-version", s.mgmt.GetLatestVersion)

		mgmt.GET("/debug", s.mgmt.GetDebug)
		mgmt.PUT("/debug", s.mgmt.PutDebug)
		mgmt.PATCH("/debug", s.mgmt.PutDebug)

		mgmt.GET("/logging-to-file", s.mgmt.GetLoggingToFile)
		mgmt.PUT("/logging-to-file", s.mgmt.PutLoggingToFile)
		mgmt.PATCH("/logging-to-file", s.mgmt.PutLoggingToFile)

		mgmt.GET("/logs-max-total-size-mb", s.mgmt.GetLogsMaxTotalSizeMB)
		mgmt.PUT("/logs-max-total-size-mb", s.mgmt.PutLogsMaxTotalSizeMB)
		mgmt.PATCH("/logs-max-total-size-mb", s.mgmt.PutLogsMaxTotalSizeMB)

		mgmt.GET("/error-logs-max-files", s.mgmt.GetErrorLogsMaxFiles)
		mgmt.PUT("/error-logs-max-files", s.mgmt.PutErrorLogsMaxFiles)
		mgmt.PATCH("/error-logs-max-files", s.mgmt.PutErrorLogsMaxFiles)

		mgmt.GET("/usage-statistics-enabled", s.mgmt.GetUsageStatisticsEnabled)
		mgmt.PUT("/usage-statistics-enabled", s.mgmt.PutUsageStatisticsEnabled)
		mgmt.PATCH("/usage-statistics-enabled", s.mgmt.PutUsageStatisticsEnabled)

		mgmt.GET("/api-keys", s.mgmt.GetAPIKeys)
		mgmt.PUT("/api-keys", s.mgmt.PutAPIKeys)
		mgmt.PATCH("/api-keys", s.mgmt.PatchAPIKeys)
		mgmt.DELETE("/api-keys", s.mgmt.DeleteAPIKeys)
		mgmt.GET("/api-key-usage", s.mgmt.GetAPIKeyUsage)

		mgmt.GET("/logs", s.mgmt.GetLogs)
		mgmt.DELETE("/logs", s.mgmt.DeleteLogs)
		mgmt.GET("/request-error-logs", s.mgmt.GetRequestErrorLogs)
		mgmt.GET("/request-error-logs/:name", s.mgmt.DownloadRequestErrorLog)
		mgmt.GET("/request-log-by-id/:id", s.mgmt.GetRequestLogByID)
		mgmt.GET("/request-log", s.mgmt.GetRequestLog)
		mgmt.PUT("/request-log", s.mgmt.PutRequestLog)
		mgmt.PATCH("/request-log", s.mgmt.PutRequestLog)
		mgmt.GET("/ws-auth", s.mgmt.GetWebsocketAuth)
		mgmt.PUT("/ws-auth", s.mgmt.PutWebsocketAuth)
		mgmt.PATCH("/ws-auth", s.mgmt.PutWebsocketAuth)

		mgmt.GET("/request-retry", s.mgmt.GetRequestRetry)
		mgmt.PUT("/request-retry", s.mgmt.PutRequestRetry)
		mgmt.PATCH("/request-retry", s.mgmt.PutRequestRetry)
		mgmt.GET("/max-retry-interval", s.mgmt.GetMaxRetryInterval)
		mgmt.PUT("/max-retry-interval", s.mgmt.PutMaxRetryInterval)
		mgmt.PATCH("/max-retry-interval", s.mgmt.PutMaxRetryInterval)

		mgmt.GET("/force-model-prefix", s.mgmt.GetForceModelPrefix)
		mgmt.PUT("/force-model-prefix", s.mgmt.PutForceModelPrefix)
		mgmt.PATCH("/force-model-prefix", s.mgmt.PutForceModelPrefix)

		mgmt.GET("/routing/strategy", s.mgmt.GetRoutingStrategy)
		mgmt.PUT("/routing/strategy", s.mgmt.PutRoutingStrategy)
		mgmt.PATCH("/routing/strategy", s.mgmt.PutRoutingStrategy)

		mgmt.GET("/claude-api-key", s.mgmt.GetClaudeKeys)
		mgmt.PUT("/claude-api-key", s.mgmt.PutClaudeKeys)
		mgmt.PATCH("/claude-api-key", s.mgmt.PatchClaudeKey)
		mgmt.DELETE("/claude-api-key", s.mgmt.DeleteClaudeKey)

		mgmt.GET("/openai-compatibility", s.mgmt.GetOpenAICompat)
		mgmt.PUT("/openai-compatibility", s.mgmt.PutOpenAICompat)
		mgmt.PATCH("/openai-compatibility", s.mgmt.PatchOpenAICompat)
		mgmt.DELETE("/openai-compatibility", s.mgmt.DeleteOpenAICompat)

		mgmt.GET("/auth-files", s.mgmt.ListAuthFiles)
		mgmt.GET("/auth-files/models", s.mgmt.GetAuthFileModels)
		mgmt.GET("/model-definitions/:channel", s.mgmt.GetStaticModelDefinitions)
		mgmt.GET("/auth-files/download", s.mgmt.DownloadAuthFile)
		mgmt.POST("/auth-files", s.mgmt.UploadAuthFile)
		mgmt.DELETE("/auth-files", s.mgmt.DeleteAuthFile)
		mgmt.PATCH("/auth-files/status", s.mgmt.PatchAuthFileStatus)
		mgmt.PATCH("/auth-files/fields", s.mgmt.PatchAuthFileFields)

		mgmt.GET("/get-auth-status", s.mgmt.GetAuthStatus)
	}
}

func (s *Server) managementAvailabilityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.managementAvailable(c) {
			return
		}
		c.Next()
	}
}

func (s *Server) managementAvailable(c *gin.Context) bool {
	if s == nil || s.cfg == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return false
	}
	if !s.managementRoutesEnabled.Load() {
		c.AbortWithStatus(http.StatusNotFound)
		return false
	}
	return true
}

func (s *Server) serveManagementControlPanel(c *gin.Context) {
	cfg := s.cfg
	if cfg == nil || cfg.RemoteManagement.DisableControlPanel {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	filePath := managementasset.FilePath(s.configFilePath)
	if strings.TrimSpace(filePath) == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			// Synchronously ensure management.html is available with a detached context.
			// Control panel bootstrap should not be canceled by client disconnects.
			if !managementasset.EnsureLatestManagementHTML(context.Background(), managementasset.StaticDir(s.configFilePath), cfg.RemoteManagement.PanelGitHubRepository) {
				c.AbortWithStatus(http.StatusNotFound)
				return
			}
		} else {
			log.WithError(err).Error("failed to stat management control panel asset")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
	}

	c.File(filePath)
}

func (s *Server) enableKeepAlive(timeout time.Duration, onTimeout func()) {
	if timeout <= 0 || onTimeout == nil {
		return
	}

	s.keepAliveEnabled = true
	s.keepAliveTimeout = timeout
	s.keepAliveOnTimeout = onTimeout
	s.keepAliveHeartbeat = make(chan struct{}, 1)
	s.keepAliveStop = make(chan struct{}, 1)

	s.engine.GET("/keep-alive", s.handleKeepAlive)

	go s.watchKeepAlive()
}

func (s *Server) handleKeepAlive(c *gin.Context) {
	if s.localPassword != "" {
		provided := strings.TrimSpace(c.GetHeader("Authorization"))
		if provided != "" {
			parts := strings.SplitN(provided, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
				provided = parts[1]
			}
		}
		if provided == "" {
			provided = strings.TrimSpace(c.GetHeader("X-Local-Password"))
		}
		if subtle.ConstantTimeCompare([]byte(provided), []byte(s.localPassword)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid password"})
			return
		}
	}

	s.signalKeepAlive()
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) signalKeepAlive() {
	if !s.keepAliveEnabled {
		return
	}
	select {
	case s.keepAliveHeartbeat <- struct{}{}:
	default:
	}
}

func (s *Server) watchKeepAlive() {
	if !s.keepAliveEnabled {
		return
	}

	timer := time.NewTimer(s.keepAliveTimeout)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			log.Warnf("keep-alive endpoint idle for %s, shutting down", s.keepAliveTimeout)
			if s.keepAliveOnTimeout != nil {
				s.keepAliveOnTimeout()
			}
			return
		case <-s.keepAliveHeartbeat:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(s.keepAliveTimeout)
		case <-s.keepAliveStop:
			return
		}
	}
}

// unifiedModelsHandler creates a unified handler for the /v1/models endpoint
// that routes to different handlers based on the User-Agent header.
// If User-Agent starts with "claude-cli", it routes to Claude handler,
// otherwise it routes to OpenAI handler.
func (s *Server) unifiedModelsHandler(openaiHandler *openai.OpenAIAPIHandler, claudeHandler *claude.ClaudeCodeAPIHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := c.Request.URL.Query()["client_version"]; ok {
			openaiHandler.OpenAIModels(c)
			return
		}

		userAgent := c.GetHeader("User-Agent")

		// Route to Claude handler if User-Agent starts with "claude-cli"
		if strings.HasPrefix(userAgent, "claude-cli") {
			// log.Debugf("Routing /v1/models to Claude handler for User-Agent: %s", userAgent)
			claudeHandler.ClaudeModels(c)
		} else {
			// log.Debugf("Routing /v1/models to OpenAI handler for User-Agent: %s", userAgent)
			openaiHandler.OpenAIModels(c)
		}
	}
}

// Start begins listening for and serving HTTP or HTTPS requests.
// It's a blocking call and will only return on an unrecoverable error.
//
// Returns:
//   - error: An error if the server fails to start
func (s *Server) Start() error {
	if s == nil || s.server == nil {
		return fmt.Errorf("failed to start HTTP server: server not initialized")
	}

	addr := s.server.Addr
	listener, errListen := net.Listen("tcp", addr)
	if errListen != nil {
		return fmt.Errorf("failed to start HTTP server: %v", errListen)
	}

	log.Debugf("Starting API server on %s", addr)

	httpListener := newMuxListener(listener.Addr(), 1024)
	s.muxBaseListener = listener
	s.muxHTTPListener = httpListener

	httpErrCh := make(chan error, 1)
	acceptErrCh := make(chan error, 1)

	go func() {
		httpErrCh <- s.server.Serve(httpListener)
	}()
	go func() {
		acceptErrCh <- s.acceptMuxConnections(listener, httpListener)
	}()

	select {
	case errServe := <-httpErrCh:
		if s.muxBaseListener != nil {
			if errClose := s.muxBaseListener.Close(); errClose != nil && !errors.Is(errClose, net.ErrClosed) {
				log.Debugf("failed to close shared listener after HTTP serve exit: %v", errClose)
			}
		}
		if s.muxHTTPListener != nil {
			_ = s.muxHTTPListener.Close()
		}
		errAccept := <-acceptErrCh
		errServe = normalizeHTTPServeError(errServe)
		errAccept = normalizeListenerError(errAccept)
		if errServe != nil {
			return fmt.Errorf("failed to start HTTP server: %v", errServe)
		}
		if errAccept != nil {
			return fmt.Errorf("failed to start HTTP server: %v", errAccept)
		}
		return nil
	case errAccept := <-acceptErrCh:
		if s.muxHTTPListener != nil {
			_ = s.muxHTTPListener.Close()
		}
		if s.muxBaseListener != nil {
			if errClose := s.muxBaseListener.Close(); errClose != nil && !errors.Is(errClose, net.ErrClosed) {
				log.Debugf("failed to close shared listener after accept loop exit: %v", errClose)
			}
		}
		errServe := <-httpErrCh
		errServe = normalizeHTTPServeError(errServe)
		errAccept = normalizeListenerError(errAccept)
		if errAccept != nil {
			return fmt.Errorf("failed to start HTTP server: %v", errAccept)
		}
		if errServe != nil {
			return fmt.Errorf("failed to start HTTP server: %v", errServe)
		}
		return nil
	}
}

// Stop gracefully shuts down the API server without interrupting any
// active connections.
//
// Parameters:
//   - ctx: The context for graceful shutdown
//
// Returns:
//   - error: An error if the server fails to stop
func (s *Server) Stop(ctx context.Context) error {
	log.Debug("Stopping API server...")

	if s.keepAliveEnabled {
		select {
		case s.keepAliveStop <- struct{}{}:
		default:
		}
	}

	if s.muxHTTPListener != nil {
		_ = s.muxHTTPListener.Close()
	}
	if s.muxBaseListener != nil {
		if errClose := s.muxBaseListener.Close(); errClose != nil && !errors.Is(errClose, net.ErrClosed) {
			log.Debugf("failed to close shared listener: %v", errClose)
		}
	}

	// Shutdown the HTTP server.
	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown HTTP server: %v", err)
	}

	log.Debug("API server stopped")
	return nil
}

// corsMiddleware returns a Gin middleware handler that adds CORS headers
// to every response, allowing cross-origin requests.
//
// Returns:
//   - gin.HandlerFunc: The CORS middleware handler
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "*")
		c.Header("Access-Control-Expose-Headers", corsExposedResponseHeadersJoined)

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func (s *Server) applyAccessConfig(oldCfg, newCfg *config.Config) {
	if s == nil || s.accessManager == nil || newCfg == nil {
		return
	}
	if _, err := access.ApplyAccessProviders(s.accessManager, oldCfg, newCfg); err != nil {
		return
	}
}

// UpdateClients updates the server's client list and configuration.
// This method is called when the configuration or authentication tokens change.
//
// Parameters:
//   - clients: The new slice of AI service clients
//   - cfg: The new application configuration
func (s *Server) UpdateClients(cfg *config.Config) {
	// Reconstruct old config from YAML snapshot to avoid reference sharing issues
	var oldCfg *config.Config
	if len(s.oldConfigYaml) > 0 {
		_ = yaml.Unmarshal(s.oldConfigYaml, &oldCfg)
	}

	// Update request logger enabled state if it has changed
	previousRequestLog := false
	if oldCfg != nil {
		previousRequestLog = oldCfg.RequestLog
	}
	if s.requestLogger != nil && (oldCfg == nil || previousRequestLog != cfg.RequestLog) {
		if s.loggerToggle != nil {
			s.loggerToggle(cfg.RequestLog)
		} else if toggler, ok := s.requestLogger.(interface{ SetEnabled(bool) }); ok {
			toggler.SetEnabled(cfg.RequestLog)
		}
	}

	if oldCfg == nil || oldCfg.LoggingToFile != cfg.LoggingToFile || oldCfg.LogsMaxTotalSizeMB != cfg.LogsMaxTotalSizeMB {
		if err := logging.ConfigureLogOutput(cfg); err != nil {
			log.Errorf("failed to reconfigure log output: %v", err)
		}
	}

	if s.requestLogger != nil && (oldCfg == nil || oldCfg.ErrorLogsMaxFiles != cfg.ErrorLogsMaxFiles) {
		if setter, ok := s.requestLogger.(interface{ SetErrorLogsMaxFiles(int) }); ok {
			setter.SetErrorLogsMaxFiles(cfg.ErrorLogsMaxFiles)
		}
	}

	if oldCfg == nil || oldCfg.DisableCooling != cfg.DisableCooling {
		auth.SetQuotaCooldownDisabled(cfg.DisableCooling)
	}

	if oldCfg != nil && oldCfg.DisableImageGeneration != cfg.DisableImageGeneration {
		log.Infof("disable-image-generation updated: %v -> %v", oldCfg.DisableImageGeneration, cfg.DisableImageGeneration)
	}

	if s.handlers != nil && s.handlers.AuthManager != nil {
		s.handlers.AuthManager.SetRetryConfig(cfg.RequestRetry, time.Duration(cfg.MaxRetryInterval)*time.Second, cfg.MaxRetryCredentials)
	}

	// Update log level dynamically when debug flag changes
	if oldCfg == nil || oldCfg.Debug != cfg.Debug {
		util.SetLogLevel(cfg)
	}

	prevSecretEmpty := true
	if oldCfg != nil {
		prevSecretEmpty = oldCfg.RemoteManagement.SecretKey == ""
	}
	newSecretEmpty := cfg.RemoteManagement.SecretKey == ""
	if s.envManagementSecret {
		s.registerManagementRoutes()
		if s.managementRoutesEnabled.CompareAndSwap(false, true) {
			log.Info("management routes enabled via MANAGEMENT_PASSWORD")
		} else {
			s.managementRoutesEnabled.Store(true)
		}
	} else {
		switch {
		case prevSecretEmpty && !newSecretEmpty:
			s.registerManagementRoutes()
			if s.managementRoutesEnabled.CompareAndSwap(false, true) {
				log.Info("management routes enabled after secret key update")
			} else {
				s.managementRoutesEnabled.Store(true)
			}
		case !prevSecretEmpty && newSecretEmpty:
			if s.managementRoutesEnabled.CompareAndSwap(true, false) {
				log.Info("management routes disabled after secret key removal")
			} else {
				s.managementRoutesEnabled.Store(false)
			}
		default:
			s.managementRoutesEnabled.Store(!newSecretEmpty)
		}
	}

	s.applyAccessConfig(oldCfg, cfg)
	s.cfg = cfg
	managementasset.SetCurrentConfig(cfg)
	// Save YAML snapshot for next comparison
	s.oldConfigYaml, _ = yaml.Marshal(cfg)

	s.handlers.UpdateClients(effectiveSDKConfig(cfg))

	if s.mgmt != nil {
		s.mgmt.SetConfig(cfg)
		s.mgmt.SetAuthManager(s.handlers.AuthManager)
	}

	// Count client sources from configuration and auth store.
	authEntries := 0
	if cfg != nil {
		tokenStore := sdkAuth.GetTokenStore()
		if dirSetter, ok := tokenStore.(interface{ SetBaseDir(string) }); ok {
			dirSetter.SetBaseDir(cfg.AuthDir)
		}
		authEntries = util.CountAuthFiles(context.Background(), tokenStore)
	}
	claudeAPIKeyCount := len(cfg.ClaudeKey)
	openAICompatCount := 0
	for i := range cfg.OpenAICompatibility {
		entry := cfg.OpenAICompatibility[i]
		if entry.Disabled {
			continue
		}
		openAICompatCount += len(entry.APIKeyEntries)
	}

	total := authEntries + claudeAPIKeyCount + openAICompatCount
	fmt.Printf("server clients and configuration updated: %d clients (%d auth entries + %d Claude API keys + %d OpenAI-compat)\n",
		total,
		authEntries,
		claudeAPIKeyCount,
		openAICompatCount,
	)
}

// (management handlers moved to internal/api/handlers/management)

// AuthMiddleware returns a Gin middleware handler that authenticates requests
// using the configured authentication providers. When no providers are available,
// it allows all requests (legacy behaviour).
func AuthMiddleware(manager *sdkaccess.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if manager == nil {
			c.Next()
			return
		}

		result, err := manager.Authenticate(c.Request.Context(), c.Request)
		if err == nil {
			if result != nil {
				c.Set("userApiKey", result.Principal)
				c.Set("accessProvider", result.Provider)
				if len(result.Metadata) > 0 {
					c.Set("accessMetadata", result.Metadata)
				}
			}
			c.Next()
			return
		}

		statusCode := err.HTTPStatusCode()
		if statusCode >= http.StatusInternalServerError {
			log.Errorf("authentication middleware error: %v", err)
		}
		c.AbortWithStatusJSON(statusCode, gin.H{"error": err.Message})
	}
}
