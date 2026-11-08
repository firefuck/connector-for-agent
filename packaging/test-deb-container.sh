#!/bin/sh
# Full dependency installation in an isolated Debian container. No host packages,
# user PCs, desktop sessions, gateway state or credentials are touched.
set -eu
cd "$(dirname "$0")/.."
package=$(readlink -f "${1:-dist/relaydesk_0.1.0_amd64.deb}")
test -r "$package"
case "${RELAYDESK_TEST_DISTRO:-debian}" in
 debian) test_image=debian:trixie-slim@sha256:a72b4433de1cf7d6bae0c4cee74d0547f58601a643b3375f070c51600d63b6b2 ;;
 ubuntu22) test_image=ubuntu:22.04@sha256:076e7bf82319619f28dd0aa9c02ab34b055ad37100395b4f49a2ffba04fb5e11 ;;
 *) echo 'Use RELAYDESK_TEST_DISTRO=debian or ubuntu22' >&2; exit 1 ;;
esac
# Apt uses this combined CA bundle for HTTPS when the executor has an egress
# proxy. It is mounted only for this disposable container, never copied to an
# image. The Docker client's proxy and registry configuration stay unchanged.
test_ca=${RELAYDESK_TEST_CA_BUNDLE:-/etc/ssl/certs/ca-certificates.crt}
test -r "$test_ca"
docker run --rm -i \
 --mount "type=bind,src=$package,dst=/tmp/relaydesk.deb,readonly" \
 --mount "type=bind,src=$test_ca,dst=/run/relaydesk-test-ca.pem,readonly" \
 "$test_image" sh -eu <<'TEST'
export DEBIAN_FRONTEND=noninteractive
for source in /etc/apt/sources.list /etc/apt/sources.list.d/*.sources; do
 if [ -f "$source" ]; then sed -i 's|http://|https://|g' "$source"; fi
done
apt-get -o Acquire::https::CaInfo=/run/relaydesk-test-ca.pem update
apt-get -o Acquire::https::CaInfo=/run/relaydesk-test-ca.pem install -y --no-install-recommends /tmp/relaydesk.deb desktop-file-utils
test "$(relaydesk version)" = 'RelayDesk 0.1.0'
test -s /usr/share/doc/relaydesk/copyright
grep -q 'RelayDesk Proprietary License' /usr/share/doc/relaydesk/copyright
grep -q 'Copyright 2009 The Go Authors.' /usr/share/doc/relaydesk/copyright
relaydesk license | grep -q 'RelayDesk Proprietary License'
for tool in xdotool import xclip xdg-open; do command -v "$tool" >/dev/null; done
desktop-file-validate /usr/share/applications/relaydesk.desktop
test -s /usr/share/icons/hicolor/scalable/apps/relaydesk.svg
test -x /usr/share/relaydesk/autostart-linux.sh
test -x /usr/share/relaydesk/uninstall-linux.sh
python3 -c 'import gi; gi.require_version("Gtk","3.0"); gi.require_version("Gst","1.0"); from gi.repository import Gst; Gst.init(None); assert all(Gst.ElementFactory.find(n) for n in ("pipewiresrc","pngenc","videoconvert","appsink"))'
test -r /usr/share/relaydesk/hotkey-linux.py
dpkg-query -W -f='${Status}\n' relaydesk | grep -qx 'install ok installed'
apt-get remove -y relaydesk
for file in /usr/share/doc/relaydesk/copyright /usr/bin/relaydesk /usr/share/applications/relaydesk.desktop /usr/share/icons/hicolor/scalable/apps/relaydesk.svg /usr/share/relaydesk/autostart-linux.sh /usr/share/relaydesk/uninstall-linux.sh; do
 test ! -e "$file"
done
echo 'PASS: apt dependency resolution, package installation, desktop metadata and removal.'
echo 'No GUI login, input or screenshot was exercised by this container test.'
TEST
