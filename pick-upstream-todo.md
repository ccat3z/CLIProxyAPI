# Pick Upstream Todo

Base commit: `a5cb8832`
Upstream HEAD: `4c0c6029`
Total: 109 commits

| # | SHA | Message | Status |
|---|-----|---------|--------|
| 1 | f5484b09 | fix(registry): Conform Claude models listing to Anthropic API schema | ⚠️ conflicts resolved |
| 2 | e3301ecc | fix(registry): Emit Claude model created_at as RFC 3339 string | ✅ clean |
| 3 | c354f88f | feat(api): Route Anthropic /v1/models requests to the Claude format | ⚠️ conflicts resolved |
| 4 | 1ed1f7b3 | Merge branch 'dev' into main | ✅ empty (merge of removed features) |
| 5 | 13f51d96 | fix(pluginhost): avoid holding host lock during plugin lifecycle | ❌ skipped (plugin code removed on custom branch) |
| 6 | a65ced4a | fix(management): reload plugins asynchronously after changes | ❌ skipped (code removed on custom branch) |
| 7 | 7f026e1a | Add runtime config clone | ✅ clean |
| 8 | a4756ab7 | Use config snapshots for management reload | ⚠️ conflicts resolved |
| 9 | 7b16321e | Stabilize management reload race tests | ⚠️ conflicts resolved |
| 10 | a3c87cee | Fix management reload snapshot ordering | ⚠️ conflicts resolved |
| 11 | 09596d2f | Treat loading plugins as busy | ❌ skipped (code removed on custom branch) |
| 12 | 125c0928 | Merge pull request #3872 from router-for-me/codex/pluginhost-async-reload | ✅ empty commit (code removed/already present on custom) |
| 13 | 8d2c00c1 | feat(plugin-config): update default plugin `Enabled` behavior to false and expand test coverage | ❌ skipped (code removed on custom branch) |
| 14 | b9d024af | feat(executor): handle usage limit errors and enhance retry logic | ❌ skipped (code removed on custom branch) |
| 15 | 8c6f279f | refactor(tests): remove obsolete test files and update reasoning effort logic | ⚠️ partial (applied validate.go isOpenAIFamily xai removal; conflict on test/thinking_conversion_test.go resolved by keeping custom deletion — e2e file imports removed providers; preserved apply_user_defined_test.go + reasoning_effort_test.go as sole coverage since e2e replacement doesn't exist on custom) |
| 16 | 0e81dee7 | Merge pull request #3873 from router-for-me/thinking | ✅ empty commit (duplicate of #15 partial; validate.go xai removal already applied, kimi/xai test files already removed, e2e test kept deleted — imports removed providers; old unit tests preserved as sole coverage) |
| 17 | c2967908 | feat(misc): align Antigravity runtime UA with agy CLI version sources | ❌ skipped (Antigravity code removed on custom branch) |
| 18 | 29f22acd | Merge pull request #3877 from sususu98/feat/antigravity-cli-ua-upstream-dev | ✅ empty commit (Antigravity code removed on custom branch) |
| 19 | 96a8b0cf | feat(executor): normalize reasoning text events and enhance handling logic | ❌ skipped (code removed on custom branch) |
| 20 | 644ba74b | feat(videos): implement auth binding for video requests and enhance proxy handling | ❌ skipped (VIDEOS code removed on custom branch) |
| 21 | f23fb122 | feat(translator): ensure tool uses stay adjacent to tool results in message generation | ✅ |
| 22 | acaf250f | feat(management): add test to validate priority preservation in auth file uploads | ⬜ pending |
| 23 | cde5081e | test(translator): add tests to validate omission of top-level `output_text` in OpenAI responses | ⬜ pending |
| 24 | dd49a520 | feat(translator): add tests to validate trailing assistant prefill stripping and sanitize tool call IDs | ⬜ pending |
| 25 | 78ba8ba7 | chore: remove Gemini CLI-related translator packages and logic | ⬜ pending |
| 26 | 365e8fc2 | feat(antigravity): HOME reasoning replay for Gemini models | ⬜ pending |
| 27 | 62c4b377 | Revert "feat(antigravity): HOME reasoning replay for Gemini models" | ⬜ pending |
| 28 | 292456a8 | feat(antigravity): HOME reasoning replay for Gemini models | ⬜ pending |
| 29 | b17d29ad | fix(antigravity): insert replayed functionCall before matching functionResponse | ⬜ pending |
| 30 | ef19f5fc | fix(antigravity): address review on replay call_id and args parsing | ⬜ pending |
| 31 | c55157dc | fix(antigravity): PR review replay scope, signature merge, and tool keys | ⬜ pending |
| 32 | ec8c2c29 | test(antigravity): cover invalid-signature replay cache clear | ⬜ pending |
| 33 | ac8fb970 | feat(thinking): remove `thinkingConfig` for `ModeNone` with zero budget and no level | ⬜ pending |
| 34 | c13dbcc2 | feat(translator): add test and logic to ensure `object` schemas include `properties` field | ⬜ pending |
| 35 | 41c52b9d | test(management): add concurrency test for Codex OAuth session handling | ⬜ pending |
| 36 | ae6c5eae | feat(runtime): add support for `gpt-image-1.5` and direct image API proxying | ⬜ pending |
| 37 | 052f1934 | fix(auth): classify transport errors as `home_unavailable` with retryable flag | ⬜ pending |
| 38 | 1d0551a9 | feat(config): improve config reload handling and introduce async management save hook | ⬜ pending |
| 39 | c020e2d0 | feat(translator): drop `apply_patch` custom tool in OpenAI responses | ⬜ pending |
| 40 | 893412e9 | feat(translator): normalize `service_tier` in Codex requests and add tests | ⬜ pending |
| 41 | 4926630a | feat(translator): support namespace tools in OpenAI response transformations | ⬜ pending |
| 42 | d33ac5e1 | feat(auth): add transient error cooldown configuration and adjust retry logic | ⬜ pending |
| 43 | 07c297a5 | feat(auth): add persistent cooldown state management with file-backed store | ⬜ pending |
| 44 | aed54adb | feat(translator): preserve structured `tool_choice` in OpenAI response conversions | ⬜ pending |
| 45 | aa2ad995 | feat(translator): preserve `input_image` details in OpenAI response conversion | ⬜ pending |
| 46 | 041a065b | Merge branch 'remove-gemini-cli' into dev | ⬜ pending |
| 47 | 1b849b6d | feat(translator): attach `reasoning_content` to assistant and tool messages in OpenAI response conversion | ⬜ pending |
| 48 | 15817006 | chore(deps): bump `github.com/jackc/pgx/v5` from v5.7.6 to v5.9.2 | ⬜ pending |
| 49 | 51aa5ba9 | feat(translator): preserve `input_audio` fields in OpenAI request conversions | ⬜ pending |
| 50 | 34639c3c | feat(translator): defer Codex function call starts until function name is available | ⬜ pending |
| 51 | c44d4fcc | feat(schema): add removal of `$comment` and `enumDescriptions` in JSON schema processing | ⬜ pending |
| 52 | 4c78e40d | feat(auth): unify provider key handling with OpenAI compatibility support | ⬜ pending |
| 53 | 75fa6265 | feat(executor): normalize `parallel_tool_calls` based on `tools` presence | ⬜ pending |
| 54 | bc652c7b | feat(translator): add support for `text.format` conversion in OpenAI to Gemini requests | ⬜ pending |
| 55 | 28e2f979 | feat(executor): add session isolation for `grok-composer` models | ⬜ pending |
| 56 | 379167c9 | feat(translator): add benchmarking for `convertSystemRoleToDeveloper` with large inputs | ⬜ pending |
| 57 | f66376f0 | feat(auth): add per-auth OAuth model alias support | ⬜ pending |
| 58 | 790ec307 | feat(config): add support for `rebuild_mid_system_message` configuration | ⬜ pending |
| 59 | b0ca3794 | Merge pull request #3834 from dcrdev/main | ⬜ pending |
| 60 | a79ae80f | feat(registry): improve model fallback logic and refactor Claude model handling | ⬜ pending |
| 61 | b4bec344 | feat(translator): sanitize `parametersJsonSchema` in OpenAI to Gemini request handling | ⬜ pending |
| 62 | 5771abbc | feat(management): add `ResetQuota` endpoint and auth manager quota reset functionality | ⬜ pending |
| 63 | 011ffe1d | feat(translator): enforce FIFO order in tool call ID consumption for Gemini requests | ⬜ pending |
| 64 | 57e1bf97 | feat(translator): ensure preservation of tool and call IDs in Gemini request and response translations | ⬜ pending |
| 65 | 09179a70 | feat(registry): add "max" level and remove deprecated Gemini models | ⬜ pending |
| 66 | 35c3d80a | feat(translator): add support for handling video URLs in Gemini requests | ⬜ pending |
| 67 | eb8d0d06 | Merge pull request #3900 from sususu98/fix/antigravity-replay-fc-order-upstream-dev | ⬜ pending |
| 68 | bb414de3 | feat(api): add "max" reasoning depth and `service_tiers` to Codex client models | ⬜ pending |
| 69 | 9a8098d2 | feat(api): prioritize non-template Codex client models and adjust priority calculation logic | ⬜ pending |
| 70 | 1f21f946 | feat(api): implement support for multi-auth expansion in plugin systems | ⬜ pending |
| 71 | 31549af1 | fix(watcher): update Gemini provider name to "gemini-cli" in file synthesizer logic | ⬜ pending |
| 72 | 5bc0c682 | feat(pluginhost): improve error handling with HTTP status codes for plugin calls | ⬜ pending |
| 73 | babef2a1 | feat(cliproxy): add `unregisterOpenAICompatExecutor` and sync runtime configuration | ⬜ pending |
| 74 | 369e560f | feat(api): refactor provider key logic for API key usage and add test for compatibility grouping | ⬜ pending |
| 75 | 1f2504eb | fix(claude): bypass signature sanitizer for non-Claude models (#3946) | ⬜ pending |
| 76 | 079ec51f | feat(cliproxy): optimize API key alias rebuild with deferred execution and caching | ⬜ pending |
| 77 | 36ed0e5c | fix(codex): strip model prefix for websocket payloads | ⬜ pending |
| 78 | 290f421f | Merge pull request #3959 from fdreamsu/codex/fix-codex-ws-prefix | ⬜ pending |
| 79 | c58da381 | feat(plugins): sync home plugin manifests | ⬜ pending |
| 80 | 5d9ea166 | Merge pull request #3963 from router-for-me/home | ⬜ pending |
| 81 | bd646819 | test(translator, runtime): ensure empty text parts are skipped without null values | ⬜ pending |
| 82 | 7c390a7a | feat(runtime): add Claude Code session handling with caching and tests | ⬜ pending |
| 83 | f1ed8912 | feat(translator): wrap message-level system roles as user-visible reminders | ⬜ pending |
| 84 | 53a21dfb | [codex] Drop foreign encrypted_content before xAI Grok upstream (#3961) | ⬜ pending |
| 85 | 05d1792d | feat(xai): replay Grok reasoning for Claude messages (#3962) | ⬜ pending |
| 86 | e9a11db7 | feat(home): enhance plugin management and synchronization | ⬜ pending |
| 87 | b89c594a | Merge pull request #3973 from router-for-me/home | ⬜ pending |
| 88 | 70053bea | feat(auth): refactor credential kind detection and add dynamic source classification | ⬜ pending |
| 89 | 3a13865d | refactor(home): relocate and rename home plugin status reporting logic | ⬜ pending |
| 90 | 38ed7aef | fix(codex): sanitize downstream UA for direct image calls | ⬜ pending |
| 91 | fd93ee03 | feat(oauth): add force-mapping for upstream response model rewrite | ⬜ pending |
| 92 | c20ac28a | Merge pull request #3981 from sususu98/feat/oauth-model-alias-force-mapping | ⬜ pending |
| 93 | 87e6d9cf | feat(videos): add model binding and propagation for video auth management | ⬜ pending |
| 94 | a183e729 | feat(auth): add ParseAuths method for expanding credential payloads into multiple auth records | ⬜ pending |
| 95 | a7250275 | fix(oauth): force-map responses to config alias not request suffix (#3983) | ⬜ pending |
| 96 | df10a5b1 | feat(pluginhost): add shadow plugin management and cleanup functionality | ⬜ pending |
| 97 | 7712ffed | Merge pull request #3986 from router-for-me/sdk | ⬜ pending |
| 98 | b53d1e95 | refactor(pluginhost): replace `Snapshot().records` with `activeRecords` for improved filtering | ⬜ pending |
| 99 | 810abe5e | feat(pluginhost): add OAuthProvider field to plugin metadata and update related functionality | ⬜ pending |
| 100 | 29b53434 | Merge pull request #3998 from router-for-me/feat/plugin-OAuth | ⬜ pending |
| 101 | 192888f9 | feat(pluginhost): enhance logging with plugin name and path fields | ⬜ pending |
| 102 | c4cf0fd3 | Merge pull request #4001 from router-for-me/plugin | ⬜ pending |
| 103 | eb2e1e33 | fix(auth): rewrite API key alias response models (#4002) | ⬜ pending |
| 104 | 7d1d2512 | docs: add Universal Chat Provider to "Who is with us?" | ⬜ pending |
| 105 | abe68cc1 | Merge pull request #4003 from maxdewald/add-universal-chat-provider | ⬜ pending |
| 106 | cb6992ef | docs: add Universal Chat Provider section to README files | ⬜ pending |
| 107 | 65f2288a | feat(models): refine Gemini 3.5 Flash variants and add Medium tier | ⬜ pending |
| 108 | 6a59d645 | feat(pluginhost): enhance plugin version management and logging for hot reload | ⬜ pending |
| 109 | 4c0c6029 | Merge pull request #4009 from router-for-me/plugin | ⬜ pending |
