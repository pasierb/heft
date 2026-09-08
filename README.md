# heft

`heft` is an opinionated, scriptable way to work on several tasks at once with
Git worktrees and Herdr.

Each task gets its own branch, worktree, and Herdr workspace. A single command
can also start your preferred agent and send it the task, while your current
workspace stays available for everything else.

## Requirements

- Git
- Herdr
- Go 1.26 or newer to build from source

## Quick start

Build and install `heft` for the current user:

```sh
make install
```

Initialize it in a Git repository:

```sh
heft init
```

Create a worktree and Herdr workspace for a task:

```sh
heft work feature/abc
```

This fetches `origin`, creates `feature/abc` from the configured base branch,
checks it out at `.worktrees/feature_abc`, and opens a Herdr workspace rooted
there. If the branch already exists locally or on `origin`, heft reuses it.

List the repository's worktrees:

```sh
heft list
```

Remove a worktree when it has no staged, unstaged, or untracked changes:

```sh
heft cleanup feature/abc
```

The local branch is preserved. To remove every clean linked worktree while
leaving dirty worktrees and local branches untouched, run:

```sh
heft prune
```

## Scriptable workflows

`heft work` is designed to sit behind small project-specific commands. This
repository uses the following Make target to start work on a Fizzy card:

```sh
make work-on-fizzy 33
```

It runs the equivalent of:

```sh
heft work fizzy-33 --prompt="Check the fizzy card id=33, analyze it and prepare solution plan"
```

That creates the worktree and workspace, starts the command configured for the
first tab, waits for Herdr to recognize it as an agent, and sends the prompt.
`heft work` returns after Herdr accepts the prompt; it does not wait for the
agent to finish.

Use `--no-focus` when a script should keep the current workspace focused:

```sh
heft work feature/abc --no-focus
```

## Configuration

`heft init` creates `.heft.yaml` at the Git project root and prompts for each
setting. Run `heft configure` later to change them.

```yaml
worktrees_dir: .worktrees
base_branch: main
workspace_prefix: heft
tabs:
  - name: codex
    command: codex
  - name: shell
```

- `worktrees_dir` controls where worktrees are stored. The default is
  `.worktrees`; heft also adds the directory to `.gitignore`.
- `base_branch` is the branch used for new worktrees. The default is `main`.
- `workspace_prefix` prefixes Herdr workspace names. It defaults to the
  repository name, producing names such as `heft feature/abc`.
- `tabs` is an ordered list of Herdr tabs. Each tab needs a `name`; an optional
  `command` runs in its root pane.

If `tabs` is omitted or empty, heft creates a single default Herdr workspace.
New configuration writes that default explicitly as `tabs: [{name: shell}]`.
Additional tabs open without stealing focus from the first tab.

`--prompt` requires the first configured tab to start a Herdr-recognized agent.
In the example above, that is `codex`.

## Commands

| Command | Description |
| --- | --- |
| `heft init` | Check for Herdr and create the project configuration |
| `heft configure` | Update the project configuration interactively |
| `heft work <branch>` | Create or reuse a branch, worktree, and Herdr workspace |
| `heft work <branch> --prompt <text>` | Start and prompt the agent in the first configured tab |
| `heft work <branch> --no-focus` | Keep the current Herdr workspace focused |
| `heft list` | List the repository's worktrees |
| `heft cleanup <branch>` | Close its Herdr workspace and remove a clean worktree |
| `heft prune` | Remove all clean linked worktrees |
| `heft version` / `heft --version` | Print version information |

Run `heft <command> --help` for command-specific usage.

## Development

```sh
make build   # build bin/heft
make run     # run without installing
make test    # run the test suite
```

`make build` derives the version from Git. Direct `go build` invocations report
`dev` unless a version is supplied with `-ldflags "-X main.version=<version>"`.
