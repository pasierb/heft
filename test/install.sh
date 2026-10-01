#!/bin/sh
# Run after make build VERSION=v0.0.0-test. No network or Herdr required.
set -eu

root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
mkdir -p "$tmp/bin" "$tmp/release" "$tmp/payload"
cp "$root/bin/heft" "$tmp/payload/heft"

# Limit PATH so a host's extra checksum utilities cannot hide a broken fallback.
for tool in curl tar gzip awk mktemp rm mkdir cp chmod; do
	ln -s "$(command -v "$tool")" "$tmp/bin/$tool"
done
cat > "$tmp/bin/uname" <<'EOF'
#!/bin/sh
case "$1" in
	-s) printf '%s\n' "$HEFT_TEST_OS" ;;
	-m) printf '%s\n' "$HEFT_TEST_ARCH" ;;
esac
EOF
chmod 755 "$tmp/bin/uname"

export HEFT_VERSION=v0.0.0-test
export HEFT_RELEASE_URL="file://$tmp/release"
export HEFT_INSTALL_DIR="$tmp/install with spaces"

check_install() {
	expected_error=$1
	if output=$(PATH="$tmp/bin" /bin/sh "$root/install.sh" 2>&1); then
		[ -z "$expected_error" ] || { printf 'Expected failure: %s\n' "$expected_error"; exit 1; }
		[ "$("$HEFT_INSTALL_DIR/heft" --version)" = "heft $HEFT_VERSION" ]
	else
		[ -n "$expected_error" ] || { printf '%s\n' "$output"; exit 1; }
		case "$output" in
			*"$expected_error"*) ;;
			*) printf 'Unexpected error: %s\n' "$output"; exit 1 ;;
		esac
		[ "$(cat "$HEFT_INSTALL_DIR/heft")" = "keep existing installation" ]
	fi
}

tested=0
for tool in sha256sum shasum; do
	command -v "$tool" >/dev/null 2>&1 || continue
	tested=$((tested + 1))
	ln -s "$(command -v "$tool")" "$tmp/bin/$tool"
	for platform in Linux:linux:x86_64:amd64 Linux:linux:aarch64:arm64 Darwin:darwin:x86_64:amd64 Darwin:darwin:arm64:arm64; do
		IFS=: read -r HEFT_TEST_OS os HEFT_TEST_ARCH arch <<EOF
$platform
EOF
		export HEFT_TEST_OS HEFT_TEST_ARCH
		archive="heft_${HEFT_VERSION}_${os}_${arch}.tar.gz"
		tar -C "$tmp/payload" -czf "$tmp/release/$archive" heft
		if [ "$tool" = sha256sum ]; then
			digest=$(sha256sum "$tmp/release/$archive" | awk '{print $1}')
		else
			digest=$(shasum -a 256 "$tmp/release/$archive" | awk '{print $1}')
		fi
		printf '%s  ./%s\n' "$digest" "$archive" > "$tmp/release/SHA256SUMS"
		check_install ""
		rm "$HEFT_INSTALL_DIR/heft"
		printf '%s\n' 'keep existing installation' > "$HEFT_INSTALL_DIR/heft"
		printf '%064d  %s\n' 0 "$archive" > "$tmp/release/SHA256SUMS"
		check_install 'checksum verification failed'
		: > "$tmp/release/SHA256SUMS"
		check_install 'release checksum not found'
	done
	(HEFT_TEST_OS=unsupported check_install 'unsupported operating system')
	(HEFT_TEST_ARCH=unsupported check_install 'unsupported architecture')
	rm "$tmp/bin/$tool"
done
[ "$tested" -gt 0 ] || { echo 'Tests require sha256sum or shasum'; exit 1; }
check_install 'requires sha256sum or shasum'
printf 'Installer checks passed (%s checksum tools).\n' "$tested"
