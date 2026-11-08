param([switch]$Persistent,[switch]$Uninstall,[switch]$ForgetPairing)
$ErrorActionPreference='Stop'
$dir=Join-Path $env:LOCALAPPDATA 'RelayDesk'
$link=Join-Path ([Environment]::GetFolderPath('Startup')) 'RelayDesk.lnk'
$exe=Join-Path $dir 'relaydesk.exe'
$notices=@('LICENSE','NOTICE','THIRD_PARTY_NOTICES.md')
if($ForgetPairing -and -not $Uninstall){throw '-ForgetPairing requires -Uninstall'}
if($Uninstall){
 Remove-Item $link -ErrorAction SilentlyContinue
 $running=Get-Process -Name relaydesk -ErrorAction SilentlyContinue | Where-Object {$_.Path -eq $exe}
 if($running){throw 'Autostart removed. Close the installed RelayDesk connector window, then rerun -Uninstall.'}
 if(Test-Path $exe){Remove-Item $exe -Force}
 foreach($name in $notices){$file=Join-Path $dir $name; if(Test-Path $file){Remove-Item $file -Force}}
 if((Test-Path $dir) -and -not (Get-ChildItem $dir -Force | Select-Object -First 1)){Remove-Item $dir}
 if($ForgetPairing){
  $configDir=Join-Path ([Environment]::GetFolderPath('ApplicationData')) 'relaydesk'
  $config=Join-Path $configDir 'connector.json'
  if(Test-Path $config){Remove-Item $config -Force}
  if((Test-Path $configDir) -and -not (Get-ChildItem $configDir -Force | Select-Object -First 1)){Remove-Item $configDir}
  Write-Host 'Local pairing removed. Revoke this device in the gateway dashboard as well.'
 }else{Write-Host 'Pairing credentials preserved. Add -ForgetPairing to remove them explicitly.'}
 Write-Host 'Autostart and installed application removed.'
 exit
}
foreach($name in $notices){
 if(-not (Test-Path (Join-Path $PSScriptRoot "..\dist\$name") -PathType Leaf)){throw "Missing distribution notice: $name. Run packaging/build.sh first."}
}
New-Item -ItemType Directory -Force $dir | Out-Null
Copy-Item (Join-Path $PSScriptRoot '..\dist\relaydesk.exe') $exe -Force
foreach($name in $notices){Copy-Item (Join-Path $PSScriptRoot "..\dist\$name") (Join-Path $dir $name) -Force}
$userSid=[System.Security.Principal.WindowsIdentity]::GetCurrent().User.Value
$acl=& icacls.exe $dir /inheritance:r /grant:r "*${userSid}:(OI)(CI)F" '*S-1-5-18:(OI)(CI)F'
if($LASTEXITCODE -ne 0){throw 'Could not restrict application folder permissions'}
if($Persistent){$w=New-Object -ComObject WScript.Shell;$s=$w.CreateShortcut($link);$s.TargetPath=$exe;$s.Arguments='connector --autostart';$s.WorkingDirectory=$dir;$s.Save()}
Write-Host "Installed at $dir. Pairing wizard opens now. Persistent mode starts after user login; it does not unlock Windows."
& $exe
