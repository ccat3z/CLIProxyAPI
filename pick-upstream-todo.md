# Pick Upstream Todo

Base commit: `2fa4dabe`
Upstream HEAD: `5afc0f1d`
Total: 35 commits

| # | SHA | Message | Status |
|---|-----|---------|--------|
| 1 | 2fa4dabe | feat(executor): improve downstream response ID rewrite and add test for repeated response scenarios | ⚠️ picked (conflict: modify/delete on removed xai files, kept deleted) |
| 2 | b05a27e4 | docs(README): add CyberPay to partners section with multilingual updates | ⚠️ picked (conflict: README_CN.md/README_JA.md deleted on custom, kept deleted; applied README.md change) |
| 3 | 21d8164c | feat(handlers): add `disable-cooling` support in OpenAI compatibility configuration | ✅ done |
| 4 | 1f16e87e | feat(pluginstore): introduce support for direct install type and version management | ⚠️ conflict |
| 5 | 884fc3ce | Merge pull request #4035 from router-for-me/plugin | ⚠️ picked (conflict: modify/delete on removed plugin files, kept deleted; sdkpluginstore import in config.go reverted) |
| 6 | 60eae92b | feat(plugins): enhance plugin deletion test and config handling | ⚠️ picked (conflict: modify/delete on removed plugins_test.go, kept deleted; applied generic pruneMappingToGeneratedKeys variadic improvement in config.go) |
| 7 | f106c416 | Merge pull request #4036 from router-for-me/plugins | ⚠️ picked (empty diff after removing deleted plugins_test.go; created empty commit with original message + author) |
| 8 | 00c0b4d7 | feat(auth): refactor authentication handling for plugins and add tests | ⚠️ picked (conflict: modify/delete on removed plugin-store files, kept deleted; created empty commit with original message + author) |
| 9 | c22795af | Merge pull request #4038 from router-for-me/plugins | ⚠️ picked (conflict: modify/delete on removed plugin files auth.go/auth_test.go/plugin_store.go/pluginstore.go, kept deleted; empty commit with original message + author) |
| 10 | 3ea7f189 | feat(pluginstore): add API URL to ReleaseAsset and update asset download logic | ⚠️ picked (conflict: modify/delete on removed pluginstore github.go/install_test.go, kept deleted; empty commit with original message + author) |
| 11 | 89708731 | feat(auth): streamline GitHub token handling and enhance download asset logic | ⚠️ picked (conflict: modify/delete on removed pluginstore auth.go/auth_test.go/github.go/install_test.go, kept deleted; verified this is pluginstore-internal GitHub-token auth, not shared auth infra; empty commit with original message + author) |
| 12 | caf70529 | feat(pluginstore): refactor installation tests to improve asset download logic and error handling | ⚠️ picked (conflict: modify/delete on removed pluginstore install_test.go, kept deleted; empty commit with original message + author) |
| 13 | dc43747c | Merge pull request #4039 from router-for-me/plugin | ⚠️ picked (conflict: modify/delete on removed pluginstore auth.go/auth_test.go/github.go/install_test.go, kept deleted; empty commit with original message + author) |
| 14 | c48516c5 | feat(tests): refactor snapshot handling in model registration tests for improved clarity and consistency | ⚠️ picked (conflict: modify/delete on removed internal/pluginhost/adapters_test.go, kept deleted; empty commit with original message + author) |
| 15 | 4b51f85c | fix(translator): map OpenAI Responses reasoning to Gemini two-part signatures | ⚠️ picked (conflict: modify/delete on removed antigravity test files and gemini translator files, kept deleted; empty commit with original message + author) |
| 16 | 3648bc15 | fix(translator): align reasoning merge with Responses visible text rules | ⚠️ picked (conflict: modify/delete on removed gemini translator files, kept deleted; empty commit with original message + author) |
| 17 | ca7478a1 | fix(antigravity): align CLI User-Agent with agy 1.0.13 short form (#4045) | ⚠️ picked (conflict: modify/delete on removed antigravity files, kept deleted; empty commit with original message + author) |
| 18 | 8c8009c1 | Merge pull request #4042 from router-for-me/plugin | ⚠️ picked (conflict: modify/delete on removed internal/pluginhost/adapters_test.go, kept deleted; empty commit with original message + author) |
| 19 | a26d3845 | Merge pull request #4043 from sususu98/fix/responses-gemini-reasoning-signature-upstream-dev | ⚠️ picked (conflict: modify/delete on removed antigravity/gemini openai-responses files, kept deleted; empty commit with original message + author) |
| 20 | 150e7f0d | fix(auth): repair force-mapped Responses SSE framing for WS forwarder | ⚠️ picked (conflict: modify/delete on removed response_model_rewriter.go and tests, kept deleted; conductor.go context-missing hunk for rewriteForceMappedStreamChunk/finishForceMappedStreamChunks discarded — surrounding force-map code path removed on custom; empty commit with original message + author) |
| 21 | 95b7cd42 | Merge pull request #4051 from sususu98/codex/fix-force-mapped-antigravity-sse-rewriter | ⬜ pending |
| 22 | 8f686345 | fix(responses): full transcript replay on WS-to-SSE Codex paths | ⬜ pending |
| 23 | 00114bec | Merge pull request #4052 from sususu98/fix/responses-ws-to-sse-4048 | ⬜ pending |
| 24 | 611d65ea | Improve reasoning content handling in response logic | ⬜ pending |
| 25 | 956ce7cf | fix(registry): add Claude Sonnet 5 model metadata | ⬜ pending |
| 26 | e681910c | Merge pull request #4069 from TooYoungTooSimp/patch-1 | ⬜ pending |
| 27 | e1302645 | feat(plugin): add methods for auth provider handling and plugin metadata retrieval | ⬜ pending |
| 28 | cde9336b | Merge pull request #4080 from router-for-me/plugin | ⬜ pending |
| 29 | c1b952da | feat(docs): add Claude API sponsorship information to README files | ⬜ pending |
| 30 | 00787ef9 | fix(docs): correct link formatting for Claude API sponsorship in README | ⬜ pending |
| 31 | 87c091e2 | fix(docs): correct formatting and wording for Claude API sponsorship in README files | ⬜ pending |
| 32 | ac21758e | feat(docs): add Code0 sponsorship information to README files in English, Chinese, and Japanese | ⬜ pending |
| 33 | f8334be8 | docs(README): update VisionCoder URLs in all language versions | ⬜ pending |
| 34 | 9e9c2442 | Merge pull request #4095 from router-for-me/readme-add | ⬜ pending |
| 35 | 5afc0f1d | fix(translator): remove temperature parameter handling in Claude request transformations | ⬜ pending |
