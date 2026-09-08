# heft

`heft` helps you work on several tasks at once with Git worktrees and Herdr.
It gives each task a branch, a worktree, and a Herdr workspace, following one
consistent workflow that is easy to use from scripts.

Pass a prompt when you create the worktree and heft will start your agent there.
Your current workspace stays open, so you can move between tasks without
shuffling branches or local changes.

## Requirements

- Git
- Herdr
- curl, tar, and sha256sum to install a release
- Go 1.26 or newer only when building from source

## Quick start

Install the latest release for the current user:

```sh
curl -fsSL https://raw.githubusercontent.com/pasierb/heft/main/install.sh | sh
```

To build and install from source instead:

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

Heft fetches `origin`, creates `feature/abc` from the configured base branch,
checks it out at `.worktrees/feature_abc`, and opens a Herdr workspace there.
If the branch already exists locally or on `origin`, heft reuses it.

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

Small project-specific commands can wrap `heft work`. For example, this
repository has a Make target for starting work on a Fizzy card:

```sh
make work-on-fizzy 33
```

The target runs the equivalent of:

```sh
heft work fizzy-33 --prompt="Check the fizzy card id=33, analyze it and prepare solution plan"
```

Heft creates the worktree and workspace, starts the command configured for the
first tab, waits for Herdr to recognize the agent, then sends the prompt. The
command returns once Herdr accepts the prompt. The agent continues working in
its new workspace.

Use `--no-focus` when a script should keep the current workspace focused:

```sh
heft work feature/abc --no-focus
```

## Configuration

`heft init` creates `.heft.yaml` at the Git project root and prompts for each
setting, including a default Claude, Codex, Agy, or custom agent command. Run
`heft configure` later to change the other settings. The harness prompt is
skipped once an agent tab is configured.

```yaml
worktrees_dir: .worktrees
base_branch: main
workspace_prefix: heft
tabs:
  - name: codex
    command: codex
    agent: true
  - name: shell
profiles:
  research:
    tabs:
      - name: codex
        command: codex --model gpt-5
        agent: true
      - name: notes
        command: nvim
```

- `worktrees_dir` controls where worktrees are stored. The default is
  `.worktrees`; heft also adds the directory to `.gitignore`.
- `base_branch` is the branch used for new worktrees. The default is `main`.
- `workspace_prefix` prefixes Herdr workspace names. It defaults to the
  repository name, producing names such as `heft feature/abc`.
- `tabs` is an ordered list of Herdr tabs. Each tab needs a `name`; an optional
  `command` runs in its root pane. Mark one command tab with `agent: true` to
  make it the target for `--prompt`.
- `profiles` contains named alternative tab configurations. Select one with
  `heft work <branch> --profile <name>`; without the flag, heft uses `tabs`.

With no `tabs` setting, heft creates one Herdr workspace with its default tab.
New configuration puts the selected agent first so it is focused, followed by
the shell tab. Choosing `Other` stores the entered shell command and names the
tab after its executable. Any extra tabs open in the background.

`--prompt` requires a configured tab marked with `agent: true` to start a
Herdr-recognized agent. In the example above, that is `codex`.

## Commands

| Command | Description |
| --- | --- |
| `heft init` | Check for Herdr and create the project configuration |
| `heft configure` | Update the project configuration interactively |
| `heft work <branch>` | Create or reuse a branch, worktree, and Herdr workspace |
| `heft work <branch> --prompt <text>` | Start and prompt the configured agent tab |
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
