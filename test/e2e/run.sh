#!/bin/sh
set -eu

export HOME=/tmp/home
export PATH="$HOME/.local/bin:$PATH"
mkdir -p "$HOME/.config/herdr"
cat > "$HOME/.config/herdr/config.toml" <<'EOF'
onboarding = false

[update]
version_check = false
manifest_check = false

[ui]
confirm_close = false
EOF

npx --yes skills@1.5.25 add /test/repo --skill heft --agent codex --global --yes --copy
test -f "$HOME/.agents/skills/heft/SKILL.md"
grep -q '^name: heft$' "$HOME/.agents/skills/heft/SKILL.md"

trap 'herdr server stop >/dev/null 2>&1 || true' EXIT HUP INT TERM
herdr server >/tmp/herdr.log 2>&1 &
timeout 5 sh -c 'until herdr status >/dev/null 2>&1; do sleep 0.1; done' || { cat /tmp/herdr.log; exit 1; }

HEFT_VERSION=v0.0.0-e2e HEFT_RELEASE_URL=file:///release sh /test/install.sh
[ "$(heft version)" = "heft v0.0.0-e2e" ]
[ "$(heft --version)" = "heft v0.0.0-e2e" ]
heft | grep -q 'Usage:'
for command in init configure work list cleanup prune version; do
	heft "$command" --help >/dev/null
done

git init --bare --initial-branch=main /tmp/origin.git
git init --initial-branch=main /tmp/project
git -C /tmp/project config user.name "Heft E2E"
git -C /tmp/project config user.email "heft@example.test"
touch /tmp/project/README.md
git -C /tmp/project add README.md
git -C /tmp/project commit -m initial
git -C /tmp/project remote add origin /tmp/origin.git
git -C /tmp/project push -u origin main

cd /tmp/project
printf '\n\n\n4\nsh\n' | heft init
grep -q 'command: sh' .heft.yaml
printf '\n\n\n' | heft configure

heft work cleanup-me --label "e2e cleanup" --no-focus
test -d .worktrees/cleanup-me
heft list | grep -q cleanup-me
herdr worktree list --cwd /tmp/project | grep -q '"branch":"cleanup-me"'
heft cleanup cleanup-me
test ! -e .worktrees/cleanup-me
! herdr worktree list --cwd /tmp/project | grep -q '"branch":"cleanup-me"'

heft work prune-one --no-focus
heft work prune-two --no-focus
test -d .worktrees/prune-one
test -d .worktrees/prune-two
heft prune
test ! -e .worktrees/prune-one
test ! -e .worktrees/prune-two
