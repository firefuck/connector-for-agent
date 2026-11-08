#!/usr/bin/python3
"""Install/remove only RelayDesk's GNOME pause shortcut for the current user."""
import argparse
import os
import shlex
import sys
from gi.repository import Gio

parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('--key',default='<Control><Alt>p',help='GNOME accelerator; resolve conflicts in Settings')
parser.add_argument('--binary',default='/usr/bin/relaydesk')
parser.add_argument('--config',default=os.path.join(os.environ.get('XDG_CONFIG_HOME',os.path.expanduser('~/.config')),'relaydesk','connector.json'))
parser.add_argument('--remove',action='store_true')
args=parser.parse_args()
if os.getuid()==0: sys.exit('Run as your desktop user without sudo.')
root='/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/relaydesk-pause/'
settings=Gio.Settings.new('org.gnome.settings-daemon.plugins.media-keys')
existing=list(settings.get_strv('custom-keybindings'))
shortcut=Gio.Settings.new_with_path('org.gnome.settings-daemon.plugins.media-keys.custom-keybinding',root)
if args.remove:
    settings.set_strv('custom-keybindings',[x for x in existing if x!=root])
    for field in ('name','command','binding'): shortcut.reset(field)
else:
    if not os.path.isabs(args.binary) or not os.path.isfile(args.binary): sys.exit('Provide an installed absolute --binary path.')
    shortcut.set_string('name','RelayDesk Pause')
    shortcut.set_string('command',shlex.join([args.binary,'control','--op','pause','--config',os.path.abspath(args.config)]))
    shortcut.set_string('binding',args.key)
    if root not in existing: settings.set_strv('custom-keybindings',existing+[root])
Gio.Settings.sync()
print('RelayDesk shortcut removed.' if args.remove else 'Pause shortcut configured. Test it in GNOME; conflicts or desktop policy may prevent activation. Resume stays in the local window.')
