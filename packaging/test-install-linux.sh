#!/bin/sh
# Runs real dpkg transactions in a disposable user-owned root; never touches /usr
# or the host package database. Dependency resolution and GUI login are separate.
set -eu
cd "$(dirname "$0")/.."
package=${1:-dist/relaydesk_0.1.0_amd64.deb}
if [ "$(id -u)" = 0 ]; then
 echo 'Run this test as an ordinary user.' >&2
 exit 1
fi
test_root=$(mktemp -d "${TMPDIR:-/tmp}/relaydesk-install-test.XXXXXX")
trap 'rm -rf "$test_root"' EXIT HUP INT TERM
install_root="$test_root/root"
mkdir -p "$install_root/var/lib/dpkg/updates" "$install_root/var/lib/dpkg/info"
touch "$install_root/var/lib/dpkg/status"
# Keep package documentation in this test even if the host excludes docs.
dpkg --path-include='/usr/share/doc/relaydesk/*' --root="$install_root" --log="$test_root/dpkg.log" --force-not-root --force-depends --force-bad-path --no-triggers --install "$package"
test "$("$install_root/usr/bin/relaydesk" version)" = 'RelayDesk 0.1.0'
test -r "$install_root/usr/share/icons/hicolor/scalable/apps/relaydesk.svg"
test "$(stat -c %a "$install_root/usr/share/applications/relaydesk.desktop")" = 644
desktop-file-validate "$install_root/usr/share/applications/relaydesk.desktop"
for notice in LICENSE NOTICE THIRD_PARTY_NOTICES.md; do
 cmp "$notice" "$install_root/usr/share/doc/relaydesk/$notice"
done
cat NOTICE LICENSE THIRD_PARTY_NOTICES.md > "$test_root/expected-copyright"
cmp "$test_root/expected-copyright" "$install_root/usr/share/doc/relaydesk/copyright"

export XDG_CONFIG_HOME="$test_root/config with spaces"
unset SUDO_USER
autostart="$install_root/usr/share/relaydesk/autostart-linux.sh"
uninstall="$install_root/usr/share/relaydesk/uninstall-linux.sh"
"$autostart" disable
test ! -d "$XDG_CONFIG_HOME"
if "$autostart" typo; then echo 'Invalid autostart operation accepted.' >&2; exit 1; fi
if "$autostart" enable; then echo 'Unpaired autostart accepted.' >&2; exit 1; fi
mkdir -p "$XDG_CONFIG_HOME/relaydesk"
printf '{}\n' > "$XDG_CONFIG_HOME/relaydesk/connector.json"
"$autostart" enable
desktop-file-validate "$XDG_CONFIG_HOME/autostart/relaydesk.desktop"
test "$(stat -c %a "$XDG_CONFIG_HOME/autostart/relaydesk.desktop")" = 600
grep -q '^Exec=/usr/bin/relaydesk connector --autostart$' "$XDG_CONFIG_HOME/autostart/relaydesk.desktop"
"$uninstall"
test ! -e "$XDG_CONFIG_HOME/autostart/relaydesk.desktop"
test -f "$XDG_CONFIG_HOME/relaydesk/connector.json"
printf 'keep\n' > "$XDG_CONFIG_HOME/relaydesk/unrelated.txt"
"$uninstall" --forget-pairing
test ! -e "$XDG_CONFIG_HOME/relaydesk/connector.json"
test -f "$XDG_CONFIG_HOME/relaydesk/unrelated.txt"
printf '{}\n' > "$XDG_CONFIG_HOME/relaydesk/connector.json"

dpkg --path-include='/usr/share/doc/relaydesk/*' --root="$install_root" --log="$test_root/dpkg.log" --force-not-root --force-depends --force-bad-path --no-triggers --remove relaydesk
test ! -e "$install_root/usr/bin/relaydesk"
test ! -e "$install_root/usr/share/doc/relaydesk/LICENSE"
test ! -e "$install_root/usr/share/applications/relaydesk.desktop"
test ! -e "$install_root/usr/share/icons/hicolor/scalable/apps/relaydesk.svg"
test ! -e "$install_root/usr/share/relaydesk/autostart-linux.sh"
test ! -e "$install_root/usr/share/relaydesk/uninstall-linux.sh"
test -f "$XDG_CONFIG_HOME/relaydesk/connector.json"
echo 'PASS: isolated dpkg install/remove, desktop metadata, paired autostart, explicit local cleanup.'
echo 'Dependencies were not installed; no desktop login or GUI was launched.'
