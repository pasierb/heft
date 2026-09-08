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

heft comes with sane defaults, but the following options can be changed:

- `worktrees_dir` - heft uses `<project root>/.worktrees` to store all worktrees.
