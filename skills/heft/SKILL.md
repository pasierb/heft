---
name: heft
description: Manage task-focused Git worktrees and matching Herdr workspaces with heft. Use when initializing or configuring heft, starting work on a branch, listing worktrees, or cleaning them up.
---

# Heft

Use `heft` instead of coordinating Git worktrees and Herdr workspaces separately.

## Workflow

1. Confirm the current directory belongs to a Git repository and inspect
   `.heft.yaml` when it exists. Use `heft <command> --help` for exact flags.
2. Run heft inside a Herdr terminal with `herdr` on PATH. Use `heft configure`
   to create or update `.heft.yaml` and add the worktree directory to `.gitignore`.
   `heft init` is an alias and also prompts when configuration already exists.
3. Start a task with `heft work <branch>`. Add `--label <text>` to override the
   Herdr workspace label, `--no-focus` to keep the current workspace focused,
   or `--prompt <text>` to prompt the configured agent tab.
4. Use `heft list` to inspect worktrees. Use `heft cleanup <branch>` for one
   worktree or `heft prune` for all eligible clean linked worktrees.

## Constraints

- New branches start from the configured base branch; existing local or remote
  branches are reused.
- `--prompt` requires a configured tab with `agent: true`.
- Cleanup refuses worktrees with staged, unstaged, or untracked changes and
  preserves local branches. Both cleanup and prune fetch origin once and refuse
  removal if fetching fails or commits are missing from origin branches.
- `--force` on cleanup or prune skips fetching and the unpushed-commit check.
  Dirty worktrees remain protected; prune also preserves active agent workspaces.
- Do not assume a worktree path; use `heft list` or the `worktrees_dir` setting.


### Standalone Herdr app through core

Heft also supports Herdr core terminals with `hc` on PATH. It detects
`HC_TERMINAL_ID` and validates it with `hc get`; this takes precedence over
TUI detection. No `herdr-app` CLI is required. Existing configuration and
profiles work with either integration.

Core terminals receive space/tab labels for adoption by the app. Display names
are automatic and focus is app-controlled: `--label`, `workspace_prefix`, and
configured tab names are stored as metadata, and `--no-focus` cannot guarantee
focus behavior. Core mode prints a notice about these limits.

Use `codex --no-daemon --yolo` for Codex tabs under core so commands execute
inside the core terminal. Heft does not rewrite saved agent commands.
Cleanup uses Heft labels, includes matching spaces and descendants, and defers
closing the invoking terminal until removals finish. Unmanaged layout elements
may remain. Failed launches retain worktrees and previously created terminals;
printed terminal addresses can be inspected with `hc get` and `hc read`.
