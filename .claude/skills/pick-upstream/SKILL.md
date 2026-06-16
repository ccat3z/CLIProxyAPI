---
name: pick-upstream
description: Cherry-pick upstream commits into the custom branch with full validation. Use this skill whenever the user wants to sync with upstream, pick upstream commits, update from upstream, or catch up with the main branch. Also use when the user mentions "upstream", "rebase", "cherry-pick", "sync upstream", "catch up", or "pick commits" in the context of keeping the custom branch up to date.
---

# Pick Upstream Commits

Cherry-pick upstream `main` commits into the `custom` branch one at a time, delegating each commit to a subagent with full validation.

## Prerequisites

Before starting, read these files to understand the custom branch's changes:

1. **`docs/custom/index.md`** — Master list of all custom features and where they live in the code. This is the single source of truth for what the custom branch modifies. Read it carefully; each feature section lists the specific files it touches. When resolving conflicts, consult this doc to identify which custom features are involved.
2. **`AGENTS.md`** — Project architecture, code conventions, build/test commands, and the rule that code changes on `custom` must be reflected in `docs/custom/`.

## Step 0: Create pick list and todo file

Fetch upstream and compute the commit list. The base commit is recorded in `docs/custom/index.md` as "Based on upstream commit" — read it from there, do not compute it with `git merge-base` (the merge-base drifts as picks land):

```bash
git fetch upstream
git log --oneline --reverse <base-commit>..upstream/main
```

Write the full list to `pick-upstream-todo.md` in the project root with this format:

```markdown
# Pick Upstream Todo

Base commit: `<base-commit>`
Upstream HEAD: `<upstream-head-sha>`
Total: N commits

| # | SHA | Message | Status |
|---|-----|---------|--------|
| 1 | abc1234 | fix(gemini-cli): use backend project ID | ⬜ pending |
| 2 | def5678 | feat(api): add protocol multiplexer | ⬜ pending |
| ... | | | |
```

Status values: `⬜ pending`, `🔄 in progress`, `✅ done`, `⚠️ conflict`, `❌ skipped`

This file is the single progress tracker — update it as each commit is processed.

## Step 1: Process each commit via subagent

For each commit in the pick list, spawn a subagent (Agent tool) to handle it. The subagent performs all the work for that single commit: pick, resolve conflicts, validate, update docs, and update the todo file.

### Subagent prompt template

Provide the subagent with this context:

```
You are cherry-picking one upstream commit into the custom branch.

Commit: <sha> — <message>

## Context
- Read `docs/custom/index.md` to understand all custom features and their affected files.
- Read `AGENTS.md` for code conventions and build/test commands.
- The base commit in docs/custom/index.md must be updated to <sha> after a successful pick.

## Instructions

1. Cherry-pick the commit:
   git cherry-pick --no-commit <sha>

2. If conflicts occur:
   - Read the conflicting files.
   - Re-read the relevant docs/custom/ feature docs to understand what must be preserved.
   - Resolve conflicts: preserve custom-branch logic, integrate upstream changes around it.
   - Review with `git diff --cached`.

3. Commit:
   git commit -m "<original-commit-message>"

4. Verify build:
   go build -o test-output ./cmd/server && rm test-output

5. Run unit tests:
   go test ./...

6. Run integration tests:
   CLI_PROXY_TEST_UPSTREAM_URL=http://127.0.0.1:8098/v1 \
   CLI_PROXY_TEST_UPSTREAM_KEY=sk-123 \
   pytest integration/ -v

   If the upstream API (http://127.0.0.1:8098/v1) is unavailable, STOP and ask the user for guidance. Do not skip or continue past a failed integration test without confirmation.

7. Verify config works:
   go run ./cmd/server --config data/config.yaml &
   SERVER_PID=$!
   sleep 3
   curl -s http://localhost:3456/v1/models | head -c 200
   kill $SERVER_PID 2>/dev/null

8. Update docs:
   - If the picked commit modifies code overlapping with a custom feature, update the relevant doc under docs/custom/.
   - Always update the base commit in docs/custom/index.md:
     sed -i 's/Based on upstream commit: `.*`/Based on upstream commit: `<sha>`/' docs/custom/index.md
   - git add docs/custom/index.md (and any other doc changes)
   - git commit --amend --no-edit

9. Update the todo file:
   Change the status for this commit in pick-upstream-todo.md from ⬜ to ✅ (or ⚠️ if conflicts were resolved, or ❌ if skipped).
   git add pick-upstream-todo.md
   git commit --amend --no-edit

## Report
When done, report:
- Whether the pick was clean or had conflicts (and how resolved)
- Build/test results (pass/fail/skip)
- Any docs/custom/ updates made
- Any issues encountered
```

### Concurrency

Process one commit at a time — do not run subagents in parallel. Each commit depends on the previous one being committed first. Wait for each subagent to complete and report before spawning the next one.

## Step 2: Final report

After all commits are processed, summarize:

- Total picked / skipped / with conflicts
- All skipped commits and reasons
- All test failures that were not resolved
- Current HEAD and base commit in `docs/custom/index.md`
- Any docs/custom/ updates made beyond the base-commit bump

## Conflict Resolution Guidelines

General principle: **upstream changes to shared infrastructure (new routes, new models, new executor logic) should be integrated; custom features must survive intact.**

When resolving conflicts, re-read the relevant feature doc under `docs/custom/` to understand exactly what the custom code does and why — that context is essential for correct resolution.

## When Things Go Wrong

If a commit cannot be resolved cleanly or causes persistent test failures:

1. Do NOT force-push or reset without user confirmation.
2. Document the problematic commit and the specific issue.
3. Mark it as ❌ skipped in the todo file and continue with the next commit.
4. Report all skipped commits at the end so the user can decide how to handle them.
