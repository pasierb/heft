#!/bin/sh
set -eu

REPO="pasierb/heft"
INSTALL_DIR="${HEFT_INSTALL_DIR:-$HOME/.local/bin}"

fail() {
	printf 'heft installer: %s\n' "$1" >&2
	exit 1
}

for command in curl tar sha256sum; do
	command -v "$command" >/dev/null 2>&1 || fail "requires $command"
done

case "$(uname -s)" in
	Linux) os=linux ;;
	*) fail "unsupported operating system: $(uname -s)" ;;
esac

case "$(uname -m)" in
	x86_64|amd64) arch=amd64 ;;
	aarch64|arm64) arch=arm64 ;;
	*) fail "unsupported architecture: $(uname -m)" ;;
esac

version=${HEFT_VERSION:-}
if [ -z "$version" ]; then
	release_url=$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest") ||
		fail "could not find the latest release"
	version=${release_url##*/}
fi
archive="heft_${version}_${os}_${arch}.tar.gz"
download_url=${HEFT_RELEASE_URL:-"https://github.com/$REPO/releases/download/$version"}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

printf 'Installing heft %s for %s/%s...\n' "$version" "$os" "$arch"
curl -fsSL --retry 3 "$download_url/$archive" -o "$tmp/$archive" || fail "download failed"
curl -fsSL --retry 3 "$download_url/SHA256SUMS" -o "$tmp/SHA256SUMS" || fail "checksum download failed"

expected=$(awk -v archive="$archive" '$2 == archive || $2 == "./" archive { print $1 }' "$tmp/SHA256SUMS")
[ -n "$expected" ] || fail "release checksum not found"
actual=$(sha256sum "$tmp/$archive" | awk '{ print $1 }')
[ "$actual" = "$expected" ] || fail "checksum verification failed"

tar -xzf "$tmp/$archive" -C "$tmp"
[ -f "$tmp/heft" ] || fail "release archive does not contain heft"
mkdir -p "$INSTALL_DIR"
cp "$tmp/heft" "$INSTALL_DIR/heft"
chmod 755 "$INSTALL_DIR/heft"

printf 'Installed heft to %s/heft\n' "$INSTALL_DIR"
case ":$PATH:" in
	*":$INSTALL_DIR:"*) ;;
	*) printf 'Add %s to your PATH.\n' "$INSTALL_DIR" ;;
esac
