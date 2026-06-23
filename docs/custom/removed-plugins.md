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

The following interfaces remain in the codebase but have no implementation after this removal. They serve as extension points for future use:

- `PluginInterceptorHost`, `PluginModelRouterHost`, `PluginExecutorHost` in `sdk/api/handlers/handlers.go`
- `PluginScheduler` in `sdk/cliproxy/auth/conductor.go`
- `PluginAuthParser` in `sdk/auth/filestore.go`
- `PluginHooks` in `sdk/translator/plugin_hooks.go`

The `sdk/pluginapi/` package is also kept: it is still imported by the interfaces above and by `cmd/server`.

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
