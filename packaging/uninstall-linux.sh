#!/bin/sh
set -eu
if [ "$(id -u)" = 0 ] || [ -n "${SUDO_USER:-}" ]; then
 echo 'Run as the desktop user, without sudo.' >&2
 exit 1
fi
case "${1:-}" in
 ''|--forget-pairing) ;;
 *) echo 'Usage: uninstall-linux.sh [--forget-pairing]' >&2; exit 1 ;;
esac
config_dir=${XDG_CONFIG_HOME:-"$HOME/.config"}
case "$config_dir" in /*) ;; *) echo 'XDG_CONFIG_HOME must be an absolute path.' >&2; exit 1 ;; esac
rm -f "$config_dir/autostart/relaydesk.desktop"
if [ "${1:-}" = --forget-pairing ]; then
 rm -f "$config_dir/relaydesk/connector.json"
 rmdir "$config_dir/relaydesk" 2>/dev/null || true
 echo 'Local pairing removed. Revoke this device in the gateway dashboard as well.'
else
 echo 'Pairing credentials preserved. Use --forget-pairing to remove them explicitly.'
fi
echo 'Autostart removed. Close active connector windows, then run: sudo apt remove relaydesk'
