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
