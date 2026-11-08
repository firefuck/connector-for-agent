#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
mkdir -p dist
rm -f dist/SHA256SUMS
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildvcs=false -trimpath -ldflags='-s -w' -o dist/relaydesk-linux-amd64 ./cmd/relaydesk
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -buildvcs=false -trimpath -ldflags='-s -w' -o dist/relaydesk.exe ./cmd/relaydesk
version=$(dist/relaydesk-linux-amd64 version)
version=${version#RelayDesk }
case "$version" in *[!0-9.]*|'') echo "Invalid product version" >&2; exit 1;; esac
cp LICENSE NOTICE THIRD_PARTY_NOTICES.md dist/
chmod 644 dist/LICENSE dist/NOTICE dist/THIRD_PARTY_NOTICES.md
root=$(mktemp -d "${TMPDIR:-/tmp}/relaydesk-deb.XXXXXX")
trap 'rm -rf "$root"' EXIT HUP INT TERM
mkdir -p "$root/DEBIAN" "$root/usr/share/doc/relaydesk" "$root/usr/bin" "$root/usr/share/applications" "$root/usr/share/relaydesk" "$root/usr/share/icons/hicolor/scalable/apps"
cp dist/relaydesk-linux-amd64 "$root/usr/bin/relaydesk"
cp packaging/relaydesk.desktop "$root/usr/share/applications/"
cp packaging/autostart-linux.sh packaging/uninstall-linux.sh packaging/hotkey-linux.py "$root/usr/share/relaydesk/"
cp packaging/relaydesk.svg "$root/usr/share/icons/hicolor/scalable/apps/"
cp LICENSE NOTICE THIRD_PARTY_NOTICES.md "$root/usr/share/doc/relaydesk/"
cat NOTICE LICENSE THIRD_PARTY_NOTICES.md > "$root/usr/share/doc/relaydesk/copyright"
cat > "$root/DEBIAN/control" <<CONTROL
Package: relaydesk
Version: $version
Section: net
Priority: optional
Architecture: amd64
Maintainer: RelayDesk Project <maintainer@example.invalid>
Depends: xdotool, imagemagick, xclip, xdg-utils, python3, python3-gi, gir1.2-gtk-3.0, gir1.2-gstreamer-1.0, gstreamer1.0-plugins-base, gstreamer1.0-plugins-good, gstreamer1.0-pipewire, xdg-desktop-portal
Recommends: xdg-desktop-portal-gnome
Description: Agent desktop gateway and connector for private WireGuard networks
 Supports interactive desktop control with local consent and session controls.
CONTROL
find "$root" -type d -exec chmod 755 {} +
find "$root" -type f -exec chmod 644 {} +
chmod 755 "$root/usr/bin/relaydesk" "$root/usr/share/relaydesk/autostart-linux.sh" "$root/usr/share/relaydesk/uninstall-linux.sh"
dpkg-deb --root-owner-group --build "$root" "dist/relaydesk_${version}_amd64.deb"
sh packaging/check-branding.sh
(cd dist && sha256sum relaydesk-linux-amd64 relaydesk.exe "relaydesk_${version}_amd64.deb" LICENSE NOTICE THIRD_PARTY_NOTICES.md > SHA256SUMS)
