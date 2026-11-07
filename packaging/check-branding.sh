#!/bin/sh
# Distribution consistency check, not tamper protection against a source editor.
set -eu
cd "$(dirname "$0")/.."
binary=dist/relaydesk-linux-amd64
version=$("$binary" version)
case "$version" in 'RelayDesk '[0-9]*.[0-9]*.[0-9]*) ;; *) echo 'Unexpected product identity' >&2; exit 1;; esac
version=${version#RelayDesk }
check_root=$(mktemp -d "${TMPDIR:-/tmp}/relaydesk-branding.XXXXXX")
trap 'rm -rf "$check_root"' EXIT HUP INT TERM
cat NOTICE LICENSE > "$check_root/expected-license"
"$binary" license > "$check_root/actual-license"
cmp "$check_root/expected-license" "$check_root/actual-license"
for notice in LICENSE NOTICE THIRD_PARTY_NOTICES.md; do cmp "$notice" "dist/$notice"; done
package="dist/relaydesk_${version}_amd64.deb"
test "$(dpkg-deb -f "$package" Package)" = relaydesk
test "$(dpkg-deb -f "$package" Version)" = "$version"
dpkg-deb -x "$package" "$check_root/package"
cmp "$binary" "$check_root/package/usr/bin/relaydesk"
for notice in LICENSE NOTICE THIRD_PARTY_NOTICES.md; do
 cmp "$notice" "$check_root/package/usr/share/doc/relaydesk/$notice"
done
cat NOTICE LICENSE THIRD_PARTY_NOTICES.md > "$check_root/expected-copyright"
cmp "$check_root/expected-copyright" "$check_root/package/usr/share/doc/relaydesk/copyright"
grep -qx 'Name=RelayDesk Connector' "$check_root/package/usr/share/applications/relaydesk.desktop"
echo 'PASS: compiled identity, embedded license, portable notices and DEB metadata agree.'
