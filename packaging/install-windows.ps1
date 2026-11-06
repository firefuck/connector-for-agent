param([switch]$Persistent,[switch]$Uninstall)
$ErrorActionPreference='Stop'
$dir=Join-Path $env:LOCALAPPDATA 'RelayDesk'
$link=Join-Path ([Environment]::GetFolderPath('Startup')) 'RelayDesk.lnk'
if($Uninstall){Remove-Item $link -ErrorAction SilentlyContinue;Write-Host 'Autostart removed. Close connector window before removing application files. Credentials remain in your user config directory.';exit}
New-Item -ItemType Directory -Force $dir | Out-Null
Copy-Item (Join-Path $PSScriptRoot '..\dist\relaydesk.exe') (Join-Path $dir 'relaydesk.exe') -Force
$acl=& icacls.exe $dir /inheritance:r /grant:r "${env:USERNAME}:(OI)(CI)F" 'SYSTEM:(OI)(CI)F'
if($LASTEXITCODE -ne 0){throw 'Could not restrict application folder permissions'}
if($Persistent){$w=New-Object -ComObject WScript.Shell;$s=$w.CreateShortcut($link);$s.TargetPath=Join-Path $dir 'relaydesk.exe';$s.Arguments='connector';$s.WorkingDirectory=$dir;$s.Save()}
Write-Host "Installed at $dir. Pairing wizard opens now. Persistent mode starts after user login; it does not unlock Windows."
& (Join-Path $dir 'relaydesk.exe')
