#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
mkdir -p dist
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildvcs=false -trimpath -ldflags='-s -w' -o dist/relaydesk-linux-amd64 ./cmd/relaydesk
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -buildvcs=false -trimpath -ldflags='-s -w' -o dist/relaydesk.exe ./cmd/relaydesk
root=build/deb
mkdir -p "$root/DEBIAN" "$root/usr/bin" "$root/usr/share/applications" "$root/usr/share/relaydesk"
cp dist/relaydesk-linux-amd64 "$root/usr/bin/relaydesk"
cp packaging/relaydesk.desktop "$root/usr/share/applications/"
cp packaging/autostart-linux.sh "$root/usr/share/relaydesk/"
cat > "$root/DEBIAN/control" <<'CONTROL'
Package: relaydesk
Version: 0.1.0
Section: net
Priority: optional
Architecture: amd64
Maintainer: RelayDesk Project <maintainer@example.invalid>
Depends: xdotool, imagemagick, xclip, xdg-utils
Description: Agent desktop gateway and connector for private WireGuard networks
 Supports Windows interactive sessions and Linux X11 desktop control.
CONTROL
chmod 755 "$root/usr/bin/relaydesk" "$root/usr/share/relaydesk/autostart-linux.sh"
dpkg-deb --root-owner-group --build "$root" dist/relaydesk_0.1.0_amd64.deb
