# heft

Seamless worktrees management for `herdr` users

## Requirements

- Go 1.26 or newer

## Development

Build the `heft` binary:

```sh
make build
./bin/heft --help
```

Build and install it for the current user:

```sh
make install
```

Run it without building a binary:

```sh
make run
```

Run the test suite:

```sh
make test
```

Version information is available through either interface:

```sh
./bin/heft --version
./bin/heft version
```

`make build` derives the version from Git. Direct `go build` invocations report
`dev` unless a version is supplied with `-ldflags "-X main.version=<version>"`.

## Config

`heft init` creates `.heft.yaml` at the Git project root and prompts for each
setting. Run `heft configure` later to change them.

heft comes with sane defaults, but the following options can be changed:

- `worktrees_dir` - heft uses `<project root>/.worktrees` to store all worktrees.
- `base_branch` - worktrees are based on `main`.
- `tabs` - an ordered list of Herdr tabs. Each tab requires a `name`; an optional
  `command` is run in its root pane.

For example, start Codex in the focused first tab and leave a shell ready in the
second:

```yaml
worktrees_dir: .worktrees
base_branch: main
tabs:
  - name: codex
    command: codex
  - name: shell
```

If `tabs` is omitted or empty, heft keeps the default single shell tab. Additional
configured tabs are created without changing focus from the first tab. Newly
generated configuration writes that default explicitly as `tabs: [{name: shell}]`.

## Worktrees

Create a worktree and branch from the latest configured base branch on `origin`:

```sh
heft work feature/abc
```

This creates branch `feature/abc` in `<worktrees_dir>/feature_abc` and a Herdr
workspace named `feature/abc` rooted there.

To start work immediately, pass a prompt to the Herdr-recognized agent started
by the first configured tab:

```sh
heft work feature/abc --prompt "Implement the feature"
```

The command returns after Herdr accepts the prompt; it does not wait for the
agent to finish.

List the repository's worktrees:

```sh
heft list
```

Remove a worktree after checking that it has no staged, unstaged, or untracked
changes:

```sh
heft cleanup feature/abc
```

The local branch is preserved.
