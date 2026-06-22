You are cherry-picking one upstream commit into the custom branch.

Commit: <sha> — <message>

## Context
- Read `docs/custom/index.md` to understand all custom features and their affected files.
- Read `AGENTS.md` for code conventions and build/test commands.
- CRITICAL: After every successful pick, the base commit in docs/custom/index.md MUST be updated to <sha>. This is not optional — it tracks which upstream commit the custom branch is based on and must be accurate at all times.

## Instructions

1. Cherry-pick the commit:
   git cherry-pick --no-commit <sha>
   For merge commits, use: git cherry-pick --no-commit -m 1 <sha>
   If a merge commit produces an empty diff, create an empty commit with the original message.

2. If conflicts occur:
   - Read the conflicting files.
   - Re-read the relevant docs/custom/ feature docs to understand what must be preserved.
   - Resolve conflicts: preserve custom-branch logic, integrate upstream changes around it.
   - Review with `git diff --cached`.

3. Commit:
   git commit -m "<original-commit-message>" --author "<original-author>"

4. Verify build:
   go build -o test-output ./cmd/server && rm test-output

5. Run unit tests:
   go test ./...

6. Run integration tests, you MUST execute them — this step is MANDATORY and may NEVER be skipped or marked as passed without actually running the tests:
   CLI_PROXY_TEST_UPSTREAM_MODEL=glm-5.1 \
   CLI_PROXY_TEST_UPSTREAM_MODEL_2=kimi-k2.6 \
   CLI_PROXY_TEST_UPSTREAM_URL=http://127.0.0.1:8098/v1 \
   CLI_PROXY_TEST_UPSTREAM_KEY=sk-123 \
   pytest integration/ -v
   If the upstream API is unavailable or the tests fail, STOP and ask the user for guidance.

7. Smoke test the running server (two sub-steps, both must pass):
   a. Start the server and find an available model:
      go run ./cmd/server --config data/config.yaml -port 3456 &
      SERVER_PID=$!
      sleep 3
      MODEL=$(curl -s http://localhost:3456/v1/models | python3 -c "import sys,json; data=json.load(sys.stdin); models=[m['id'] for m in data.get('data',[])]; print(models[0] if models else '')" 2>/dev/null)
      if [ -z "$MODEL" ]; then
        echo "ERROR: No models available from /v1/models"
        kill $SERVER_PID 2>/dev/null
      else
        echo "Found model: $MODEL"
      fi

   b. Use the model to test the chat completion API:
      curl -s http://localhost:3456/v1/chat/completions \
        -H "Content-Type: application/json" \
        -d "{\"model\":\"$MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"hi\"}],\"max_tokens\":5}" \
        | python3 -c "import sys,json; r=json.load(sys.stdin); print('Response OK, model:', r.get('model','?'))" 2>/dev/null || echo "ERROR: chat completion failed"
      kill $SERVER_PID 2>/dev/null

   Both sub-steps must succeed. If either fails, do not skip — report the failure and ask the user for guidance.

8. Update docs:
   - If the picked commit modifies code overlapping with a custom feature, update the relevant doc under docs/custom/.
   - ALWAYS update the base commit in docs/custom/index.md — this is mandatory for every single commit:
     sed -i 's/Based on upstream commit: `.*`/Based on upstream commit: `<sha>`/' docs/custom/index.md
   - Verify the update took effect:
     grep 'Based on upstream commit' docs/custom/index.md
   - git add -f docs/custom/index.md (and any other doc changes)
   - git commit --amend --no-edit

9. Update the todo file:
   Change the status for this commit in pick-upstream-todo.md from ⬜ to ✅ (or ⚠️ if conflicts were resolved, or ❌ if skipped).
   git add pick-upstream-todo.md
   git commit --amend --no-edit

10. Completion checklist — verify every step above was actually performed before reporting:

   | Step | Description | Done? |
   |------|-------------|-------|
   | 1 | Cherry-pick applied | ☐ |
   | 2 | Conflicts resolved (if any) | ☐ |
   | 3 | Committed with original message | ☐ |
   | 4 | Build passes (`go build`) | ☐ |
   | 5 | Unit tests pass (`go test ./...`) | ☐ |
   | 6 | Integration tests passed | ☐ |
   | 7a | Smoke test: /v1/models returns a usable model | ☐ |
   | 7b | Smoke test: chat completion succeeds with that model | ☐ |
   | 8 | Base commit updated in docs/custom/index.md and verified | ☐ |
   | 9 | Todo file status updated | ☐ |

   If any checkbox is unchecked, complete that step before reporting. No step may be skipped — if a step cannot be completed (e.g. tests fail, upstream API unavailable), report it explicitly and ask the user for guidance rather than silently moving on.

## Report
When done, report:
- Whether the pick was clean or had conflicts (and how resolved)
- Build/test results (pass/fail — no "skip")
- Whether the base commit was updated (and to what SHA)
- Any docs/custom/ updates made
- Any issues encountered
- The completed checklist
