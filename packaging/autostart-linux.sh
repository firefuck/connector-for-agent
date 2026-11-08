#!/bin/sh
set -eu
if [ "$(id -u)" = 0 ] || [ -n "${SUDO_USER:-}" ]; then
 echo 'Run as the desktop user, without sudo.' >&2
 exit 1
fi
config_dir=${XDG_CONFIG_HOME:-"$HOME/.config"}
case "$config_dir" in /*) ;; *) echo 'XDG_CONFIG_HOME must be an absolute path.' >&2; exit 1 ;; esac
target="$config_dir/autostart/relaydesk.desktop"
case "${1:-enable}" in
 enable)
  if [ ! -f "$config_dir/relaydesk/connector.json" ]; then
   echo 'Pair this desktop with relaydesk first; no connector credentials found.' >&2
   exit 1
  fi
  mkdir -p "$config_dir/autostart"
  # The login launcher never prompts for pairing and stays visible to the user.
  umask 077
  staging=$(mktemp "$config_dir/autostart/relaydesk.XXXXXX")
  trap 'rm -f "$staging"' EXIT HUP INT TERM
  cat > "$staging" <<'DESKTOP'
[Desktop Entry]
Type=Application
Name=RelayDesk Connector
Comment=Connect this paired desktop to your agent gateway
Exec=/usr/bin/relaydesk connector --autostart
TryExec=/usr/bin/relaydesk
Icon=relaydesk
Terminal=true
X-GNOME-Autostart-enabled=true
DESKTOP
  mv -f "$staging" "$target"
  echo 'Connector will start visibly after desktop login. Close its window to disconnect.'
  ;;
 disable)
  rm -f "$target"
  echo 'Autostart disabled. Stop the active connector window separately.'
  ;;
 *) echo 'Usage: autostart-linux.sh enable|disable' >&2; exit 1 ;;
esac
