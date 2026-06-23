# Removed: Plugin System

The entire plugin system is gone: the plugin host (`internal/pluginhost/`), the plugin store (`internal/pluginstore/`), all management API routes for plugins, and every integration point throughout the codebase. The `custom` branch does not load or execute plugins.

## Deleted Packages

- `internal/pluginhost/` — Plugin host lifecycle management, auth providers, model registration, management routes, executor registration, and command-line flags
- `internal/pluginstore/` — Plugin store/registry, GitHub-based installation, checksum verification, and version management

## Deleted Files

- `internal/api/handlers/management/plugins.go` — Management API handlers for plugin CRUD operations
- `internal/api/handlers/management/plugin_store.go` — Management API handlers for plugin store operations
- `internal/api/handlers/management/plugins_test.go` — Tests for plugin management handlers
- `internal/api/handlers/management/plugin_store_test.go` — Tests for plugin store handlers
- `sdk/cliproxy/service_executor_registration_test.go` — Tests for plugin executor registration
- `sdk/cliproxy/service_plugin_scheduler_test.go` — Tests for plugin scheduler injection
- `sdk/cliproxy/service_plugin_executor_test.go` — Tests for `hasNativeOpenAICompatExecutorConfig`

## Modified Files

### `sdk/cliproxy/service.go`
- Removed `pluginHost` field from `Service` struct
- Removed `registerPluginExecutors` package-level variable
- Removed `registerPluginAuthParser()` method
- Simplified `syncPluginRuntimeConfig()` to clear plugin scheduler and auth parser instead of initializing plugin host
- Simplified `syncPluginModelRuntime()` to remove plugin model registration and `refreshPluginModelRegistrations()`
- Removed `refreshPluginModelRegistrations()`, `pluginModelsForProvider()`, `appendPluginModels()`, `tryRegisterPluginModelsForAuth()`, `hasNativeOpenAICompatExecutorConfig()` methods
- Simplified `registerAvailableExecutors()` to remove plugin executor registration path
- Removed pluginHost check in `registerExecutorForAuth` default case
- Simplified `Shutdown()` to clear plugin hooks/auth parser instead of full plugin host shutdown
- Removed `tryRegisterPluginModelsForAuth` call in `registerModelsForAuth`
- Removed `appendPluginModels` calls from model registration flow

### `sdk/cliproxy/builder.go`
- Removed `pluginHost` field from `Builder` struct
- Removed `WithPluginHost()` method
- Removed plugin host creation, `ApplyConfig`, and `RegisterFrontendAuthProviders` calls from `Build()`
- Removed `pluginHost` from `Service` struct initialization
- Removed `api.WithPluginHost(pluginHost)` server option
- Removed `coreManager.SetPluginScheduler(pluginHost)` call

### `internal/api/server.go`
- Removed `pluginhost` import
- Removed `X-CPA-SUPPORT-PLUGIN` from CORS exposed headers
- Removed `pluginHost` field from `serverOptionConfig` and `Server` struct
- Removed `WithPluginHost` server option
- Removed plugin host initialization in `NewServer`
- Replaced `refreshPluginManagementRoutes()` and `pluginManagementNoRoute` with simple 404 NoRoute handler
- Removed 5 methods: `refreshPluginManagementRoutes`, `RefreshPluginManagementRoutes`, `registeredManagementRouteKeys`, `pluginManagementNoRoute`, `pluginResourceNoRoute`
- Removed plugin management route registrations (8 routes under `/v0/management/plugins/` and `/v0/management/plugin-store/`)
- Removed plugin host references from `UpdateClients`
- Removed `/v0/resource/plugins/` from heartbeat middleware path check

### `internal/api/handlers/management/handler.go`
- Removed `pluginhost` and `pluginstore` imports
- Removed `pluginHost`, `pluginStoreRegistryURL`, `pluginStoreHTTPClient`, `pluginReleaseCacheMu`, `pluginReleaseCache` fields from `Handler` struct
- Removed `SetPluginHost()` method
- Simplified `reloadConfigAfterManagementSave()` to remove pluginHost fallback
- Removed `X-CPA-SUPPORT-PLUGIN` header from `Middleware`

### `internal/api/handlers/management/auth_files.go`
- Simplified `ServePluginAuthURL()` to always return `false`
- Removed plugin auth polling code from `GetAuthStatus()`

### `cmd/server/main.go`
- Removed `pluginhost` import
- Removed plugin host creation and bootstrap config loading
- Changed `cmd.StartServiceWithPluginHost()` call to `cmd.StartService()`
- Removed `pluginBootstrapConfigPath`, `defaultPluginBootstrapConfigPath`, `loadPluginBootstrapConfig` functions

### `internal/cmd/run.go`
- Merged `StartServiceWithPluginHost` into `StartService` (removed pluginHost parameter)
- Merged `StartServiceBackgroundWithPluginHost` into `StartServiceBackground` (removed pluginHost parameter)

### Test Files Updated
- `internal/api/handlers/management/handler_test.go` — Removed `pluginhost` import; replaced `TestMiddlewareSetsSupportPluginHeader` with `TestMiddlewareSetsVersionHeaders`
- `internal/api/server_test.go` — Removed `pluginhost` import; replaced plugin host tests with 404 tests for removed routes

## Remaining Interfaces

After the host/store removal above, a layer of extension-point interfaces and the `sdk/pluginapi/` type package were left in place as stubs. None of them had any production implementation: the runtime only ever wired them to `nil`, and the only callers that exercised them were unit-test stubs. They have since been removed as well (see "Extension-point and pluginapi removal" below).

## Extension-point and pluginapi removal

The leftover plugin extension machinery has been deleted entirely. None of it was reachable at runtime: every registration site (`SetPluginHost`, `SetModelRouterHost`, `SetPluginScheduler`, `RegisterPluginAuthParser`, `SetPluginAuthParser`, `SetPluginHooks`) was only ever called with `nil` in production, and the dispatch paths all short-circuited to the native executors/translators/schedulers. The only code that exercised these interfaces was unit-test stubs, which were dropped alongside them.

### Deleted Package

- `sdk/pluginapi/` — host-side plugin capability schema (`Plugin`, `Capabilities`, request/response intercept types, scheduler types, model-route types, host model-execution request types). No code imports it after this removal.

### Deleted Files

- `sdk/translator/plugin_hooks.go` — the `PluginHooks` translator extension interface.
- `sdk/api/handlers/handlers_interceptors_test.go` — tests for the plugin request/response/stream interceptors.
- `sdk/api/handlers/handlers_model_router_test.go` — tests for the plugin model router and plugin-executor routes.
- `sdk/api/handlers/model_execution_test.go` — tests for the plugin host model-execution callback API.

### Removed Interfaces and Dispatch Code

- `sdk/api/handlers/handlers.go` — removed `PluginInterceptorHost`, `PluginModelRouterHost`, `PluginExecutorHost` and their `*Except`/detector helper interfaces; removed `BaseAPIHandler.PluginHost`/`ModelRouterHost` fields and the `SetPluginHost`/`SetModelRouterHost` setters; removed the plugin-executor execution paths (`executeWithPluginExecutor`, `countWithPluginExecutor`, `streamWithPluginExecutor`, `pluginExecutorRequest`), the model router (`applyModelRouter`, `routeModel`, `modelRoutersEnabled`, `modelRouteDecision`), and every request/response/stream interceptor helper (`applyRequestInterceptorsBeforeAuth`, `requestAfterAuthInterceptor`, `applyRequestInterceptorsAfterAuth`, `applyResponseInterceptors`, the `intercept*` helpers, `requestAfterAuthCapture`, and the now-dead stream-history/header helpers). The native Execute/Stream/Count paths are unchanged in behavior.
- `sdk/api/handlers/model_execution.go` — removed the plugin host model-execution callback API (`ExecuteModel`, `ExecuteModelStream`, `ModelExecutionRequest/Response/Stream/Chunk/Error` and their stream-wrapping helpers) and the plugin-marker fields (`SkipInterceptorPluginID`, `SkipRouterPluginID`, `InternalSource`). Only the internal `modelExecutionOptions` (headers/query) and protocol/header/query helpers remain.
- `sdk/cliproxy/auth/conductor.go` — removed `PluginScheduler`, the `pluginScheduler` field, `SetPluginScheduler`, `hasPluginScheduler`, and the scheduler dispatch (`pickViaPluginScheduler`, `pickViaBuiltinScheduler`, `builtinSchedulerStrategy`, `schedulerAuthCandidates`, `schedulerProviders`, `schedulerOptions`, `pickSchedulerAuthByID`, and the attribute/metadata cloning helpers that only served them). Auth selection now goes straight to the native selector.
- `sdk/auth/filestore.go` — removed `PluginAuthParser`, `RegisterPluginAuthParser`, and the auth-file parsing hook.
- `sdk/cliproxy/types.go`, `internal/watcher/watcher.go`, `internal/watcher/clients.go`, `internal/watcher/dispatcher.go`, `internal/watcher/synthesizer/context.go`, `internal/watcher/synthesizer/file.go` — removed the `PluginAuthParser` threading through the watcher and auth-synthesis pipeline.
- `sdk/translator/registry.go` — removed the `PluginHooks` field, `SetPluginHooks`, and the hook-driven translate/normalize fallbacks; translation now uses only the registered native transforms.

### Runtime Wiring

- `sdk/cliproxy/service.go` — replaced the no-op plugin sync layer (`syncPluginRuntime`, `syncPluginRuntimeConfig`, `syncPluginModelRuntime`) and the `includePlugins` registration flag with a single `reregisterExecutors` helper that rebuilds the executor set, and dropped the shutdown-time plugin clear calls.
- `config.example.yaml` — removed the `plugins:` configuration block. `internal/config/config.go` still strips a leftover `plugins` key from older user configs via `removeRemovedIntegrationKeys`, so existing `data/config.yaml` files keep loading unchanged.

## Routes Removed

All of the following management API routes now return 404:

- `GET /v0/management/plugins`
- `GET /v0/management/plugins/:name`
- `PATCH /v0/management/plugins/:name`
- `PUT /v0/management/plugins/:name/config`
- `PATCH /v0/management/plugins/:name/config`
- `DELETE /v0/management/plugins/:name`
- `GET /v0/management/plugin-store`
- `POST /v0/management/plugin-store/install`
