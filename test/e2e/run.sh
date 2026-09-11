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

# Operational commands must reject ordinary terminals before touching the repo.
cd /tmp/project
if env -u HERDR_WORKSPACE_ID heft configure >/tmp/outside.log 2>&1; then
    echo "configure unexpectedly ran outside Herdr" >&2
    exit 1
fi
grep -q 'inside a Herdr terminal' /tmp/outside.log
test ! -e .heft.yaml
test ! -e .gitignore

cat > /tmp/workflow.sh <<'WORKFLOW'
#!/bin/sh
set -eu
export PATH="/tmp/home/.local/bin:$PATH"
cd /tmp/project
printf 'trees\n\n\n4\nsh\n' | heft configure
grep -q 'command: sh' .heft.yaml
printf '\n\n\n' | heft configure
printf 'renamed-trees\n\n\n' | heft init
grep -q 'worktrees_dir: renamed-trees' .heft.yaml
printf 'trees\n\n\n' | heft configure

heft work cleanup-me --label "e2e cleanup" --no-focus
test -d trees/cleanup-me
heft list | grep -q cleanup-me
herdr worktree list --cwd /tmp/project | grep -q '"branch":"cleanup-me"'
(cd trees/cleanup-me && heft cleanup cleanup-me)
test ! -e trees/cleanup-me
! herdr worktree list --cwd /tmp/project | grep -q '"branch":"cleanup-me"'

# Commands inside a linked checkout use the primary configuration and paths.
heft work prune-one --label "e2e prune one" --no-focus
mkdir -p trees/prune-one/nested
(
    cd trees/prune-one/nested
    heft work prune-two --label "e2e prune two" --no-focus
    heft list | grep -q prune-two
)
test -d trees/prune-one
test -d trees/prune-two
test ! -e trees/prune-one/.worktrees
test ! -e trees/prune-one/trees
herdr workspace list | node -e '
const {workspaces} = JSON.parse(require("fs").readFileSync(0, "utf8")).result;
for (const label of ["e2e prune one", "e2e prune two"])
    require("assert").ok(workspaces.some(w => w.label === label), label + " missing");
'
# Run in the workspace being closed: closure must not interrupt removal.
workspace_id=$(herdr workspace list | node -e '
const {workspaces} = JSON.parse(require("fs").readFileSync(0, "utf8")).result;
console.log(workspaces.find(w => w.label === "e2e prune one").workspace_id);
')
pane_id=$(herdr pane list --workspace "$workspace_id" | node -e '
console.log(JSON.parse(require("fs").readFileSync(0, "utf8")).result.panes[0].pane_id);
')
heft_binary=$(command -v heft)
herdr pane run "$pane_id" "cd /tmp/project/trees/prune-one/nested && '$heft_binary' prune"
timeout 10 sh -c 'while [ -d /tmp/project/trees/prune-one ] || [ -d /tmp/project/trees/prune-two ]; do sleep 0.1; done'
test ! -e trees/prune-one
test ! -e trees/prune-two
test -d .git
test -f .heft.yaml
git show-ref --verify refs/heads/prune-one
git show-ref --verify refs/heads/prune-two
timeout 10 node --input-type=module -e '
import {execFileSync} from "node:child_process";
import {setTimeout} from "node:timers/promises";
while (JSON.parse(execFileSync("herdr", ["workspace", "list"], {encoding: "utf8"}))
    .result.workspaces.some(w => ["e2e prune one", "e2e prune two"].includes(w.label)))
    await setTimeout(100);
'

# Unpushed work retains both the worktree and its workspace until explicitly forced.
heft work unpushed --no-focus
git -C trees/unpushed commit --allow-empty -m unpushed
if heft cleanup unpushed; then
    echo "cleanup unexpectedly removed unpushed work" >&2
    exit 1
fi
heft prune
test -d trees/unpushed
herdr worktree list --cwd /tmp/project | grep -q '"branch":"unpushed"'
heft cleanup unpushed --force
test ! -e trees/unpushed
! herdr worktree list --cwd /tmp/project | grep -q '"branch":"unpushed"'
git show-ref --verify refs/heads/unpushed

WORKFLOW
pane_id=$(herdr workspace create --cwd /tmp/project --label "e2e runner" --no-focus | node -e '
console.log(JSON.parse(require("fs").readFileSync(0, "utf8")).result.root_pane.pane_id);
')
herdr pane run "$pane_id" 'sh /tmp/workflow.sh > /tmp/workflow.log 2>&1; echo $? > /tmp/workflow.status'
if ! timeout 60 sh -c 'until [ -s /tmp/workflow.status ]; do sleep 0.1; done'; then
    cat /tmp/workflow.log /tmp/herdr.log
    exit 1
fi
cat /tmp/workflow.log
[ "$(cat /tmp/workflow.status)" = 0 ]
