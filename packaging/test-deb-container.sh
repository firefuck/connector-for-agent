#!/bin/sh
# Full dependency installation in an isolated Debian container. No host packages,
# user PCs, desktop sessions, gateway state or credentials are touched.
set -eu
cd "$(dirname "$0")/.."
package=$(readlink -f "${1:-dist/relaydesk_0.1.0_amd64.deb}")
test -r "$package"
test_image=debian:trixie-slim@sha256:a72b4433de1cf7d6bae0c4cee74d0547f58601a643b3375f070c51600d63b6b2
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
sed -i 's|http://deb.debian.org|https://deb.debian.org|g' /etc/apt/sources.list.d/debian.sources
apt-get -o Acquire::https::CaInfo=/run/relaydesk-test-ca.pem update
apt-get -o Acquire::https::CaInfo=/run/relaydesk-test-ca.pem install -y --no-install-recommends /tmp/relaydesk.deb desktop-file-utils
test "$(relaydesk version)" = 'RelayDesk 0.1.0'
for tool in xdotool import xclip xdg-open; do command -v "$tool" >/dev/null; done
desktop-file-validate /usr/share/applications/relaydesk.desktop
test -s /usr/share/icons/hicolor/scalable/apps/relaydesk.svg
test -x /usr/share/relaydesk/autostart-linux.sh
test -x /usr/share/relaydesk/uninstall-linux.sh
dpkg-query -W -f='${Status}\n' relaydesk | grep -qx 'install ok installed'
apt-get remove -y relaydesk
for file in /usr/bin/relaydesk /usr/share/applications/relaydesk.desktop /usr/share/icons/hicolor/scalable/apps/relaydesk.svg /usr/share/relaydesk/autostart-linux.sh /usr/share/relaydesk/uninstall-linux.sh; do
 test ! -e "$file"
done
echo 'PASS: Debian apt dependency resolution, package installation, desktop metadata and removal.'
echo 'No GUI login, input or screenshot was exercised by this container test.'
TEST
