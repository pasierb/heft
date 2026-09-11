![heft logo](assets/logo.svg)

# heft

`heft` gives each task a Git branch, worktree, and Herdr workspace.
Pass a prompt to start your agent there. Your current workspace stays open.

![Several hefts grazing across shared hills](assets/heft-concept.jpg)

## Requirements

- Git
- Herdr

## Quick start

Install the latest release for the current user:

```sh
curl -fsSL https://raw.githubusercontent.com/pasierb/heft/main/install.sh | sh
```

## Agent skill

Teach your coding agent when and how to use heft:

```sh
npx skills add pasierb/heft --skill heft --global
```

## Usage

```sh
heft init
heft work feature/abc
heft list
heft cleanup feature/abc
heft prune
```

`work` fetches `origin`, branches from the configured base, and opens a Herdr
workspace at `.worktrees/feature_abc`. Existing local or remote branches and
worktrees are reused without changing their files or commits. Every run opens a
new workspace with the configured tabs.

Run commands from any worktree or subdirectory. Paths and `.heft.yaml` resolve
from the primary checkout; linked-worktree config is ignored. Both `init` and
`configure` update the primary checkout's config and `.gitignore`.

### Local files

Add `.heftcopy` to the primary checkout to copy local files into new worktrees:

```text
# Local setup
.heft.yaml
.env
.local/
```

Paths are literal and relative to the primary checkout. Blank lines and `#`
comments are ignored; surrounding whitespace is trimmed. Directories copy
recursively, including hidden files and permissions. Existing files are kept
and directories merged. Copying runs before Herdr opens; failures warn but don't
block it. Nothing is copied without `.heftcopy`.

No globbing, exclusions, absolute paths, parent traversal, Git metadata,
symlinks, or directories containing the destination. Add entries to `.gitignore`
separately to keep copied files untracked.

### Cleanup

`cleanup` and `prune` remove only clean worktrees (including no untracked files)
whose commits exist on any origin branch. Both fetch origin first and stop if
it's unavailable. No upstream or matching branch name is required; squash merges
may leave original commits protected.

Local branches and the primary checkout stay. `prune` also skips active agent
workspaces, but can remove the worktree you're running it from.

`--force` skips fetching and checking for unpushed commits, allowing offline
removal. It still protects dirty worktrees; pruning still skips active agents.

## Scriptable workflows

```sh
heft work fizzy-33 --prompt="Check the fizzy card id=33, analyze it and prepare solution plan"
heft work feature/abc --no-focus
heft work feature/abc --label "ticket 39"
```

`--prompt` starts the configured agent, waits for Herdr to recognize it, and
returns once Herdr accepts the prompt. The agent keeps running in its workspace.
`--no-focus` keeps your current workspace focused; `--label` overrides the label
built from the configured prefix and branch name.

This repository's `make work-on-fizzy 33` wraps the prompt command above.

## Configuration

`heft init` creates `.heft.yaml` and prompts for settings, including a Claude,
Codex, Agy, or custom agent command. `heft configure` updates settings, keeping
existing agent commands and the saved base branch default. Once an agent tab is
configured, heft skips agent selection.

```yaml
worktrees_dir: .worktrees
base_branch: main
workspace_prefix: heft
tabs:
  - name: codex
    command: codex --yolo
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

- `worktrees_dir`: defaults to `.worktrees`, added to `.gitignore`.
- `base_branch`: base for new branches. Suggested offline from local
  `origin/HEAD`, then `origin/main`, `origin/master`, local `main`, local `master`,
  or finally `main`. Override at the prompt.
- `workspace_prefix`: defaults to the repository name, e.g. `heft feature/abc`.
- `tabs`: ordered tabs with a required `name` and optional `command` run in the
  root pane. Mark one with `agent: true` for `--prompt`; its command must start
  an agent Herdr recognizes. Without `tabs`, Herdr opens its default tab.
- `profiles`: alternative tabs selected with `heft work <branch> --profile <name>`.
  Otherwise, heft uses `tabs`.

Setup focuses the agent tab, followed by a shell tab; extra tabs open in the
background. Codex defaults to `codex --yolo`, disabling approval prompts and
sandboxing. `Other` stores your command and names the tab after its executable.

## Commands

| Command | Description |
| --- | --- |
| `heft init` | Check for Herdr and create the project configuration |
| `heft configure` | Update the project configuration interactively |
| `heft work <branch>` | Create or reuse a branch, worktree, and Herdr workspace |
| `heft work <branch> --label <text>` | Set a custom Herdr workspace label |
| `heft work <branch> --prompt <text>` | Start and prompt the configured agent tab |
| `heft work <branch> --no-focus` | Keep the current Herdr workspace focused |
| `heft list` | List the repository's worktrees |
| `heft cleanup <branch>` | Close its Herdr workspace and remove a clean worktree with commits on origin |
| `heft prune` | Remove clean linked worktrees with commits on origin and without active agents |
| `heft version` / `heft --version` | Print version information |

Run `heft <command> --help` for command-specific usage.

## Development

Building from source requires Go 1.26 or newer.

```sh
make install # build and install from source
make build   # build bin/heft
make run     # run without installing
make test    # run the test suite
make test-e2e # test installation and every command in Docker with real Herdr
```

`make build` gets the version from Git. Running `go build` directly reports
`dev` unless you supply a version with `-ldflags "-X main.version=<version>"`.
