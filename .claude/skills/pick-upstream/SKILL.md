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

Read the full subagent prompt from `references/subagent-prompt.md` (relative to this skill directory). Before spawning each subagent, replace the two placeholders in that template:

- `<sha>` — the commit SHA being picked
- `<message>` — the commit message (one-line summary)

Then pass the resulting text as the subagent's prompt.

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
