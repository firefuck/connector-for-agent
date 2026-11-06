#!/bin/sh
set -eu
if [ "$(id -u)" = 0 ]; then echo 'Run as the desktop user, without sudo.' >&2; exit 1; fi
mkdir -p "$HOME/.config/autostart"
if [ "${1:-enable}" = disable ]; then
 rm -f "$HOME/.config/autostart/relaydesk.desktop"
 echo 'Autostart disabled. Stop the active connector window separately.'
else
 cp /usr/share/applications/relaydesk.desktop "$HOME/.config/autostart/relaydesk.desktop"
 echo 'Connector will start visibly after desktop login.'
fi
