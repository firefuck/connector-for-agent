; Build on Windows using Inno Setup 6: ISCC.exe packaging\relaydesk.iss
; The installer is unsigned until the owner supplies a code-signing certificate.
#ifndef ProductVersion
 #define ProductVersion "0.1.0"
#endif
[Setup]
AppId={{FB7CFDD2-65D1-4814-89D4-605B83FD12BD}
AppName=RelayDesk
AppVersion={#ProductVersion}
AppPublisher=RelayDesk
AppPublisherURL=https://github.com/firefuck/connector-for-agent
DefaultDirName={localappdata}\RelayDesk
DefaultGroupName=RelayDesk
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir=..\dist
OutputBaseFilename=RelayDesk-Setup-{#ProductVersion}-x64
LicenseFile=..\LICENSE
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
CloseApplications=yes
RestartApplications=no
UninstallDisplayIcon={app}\relaydesk.exe
[Files]
Source: "..\dist\relaydesk.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\LICENSE"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\NOTICE"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\THIRD_PARTY_NOTICES.md"; DestDir: "{app}"; Flags: ignoreversion
[Tasks]
Name: "autostart"; Description: "Mulai RelayDesk setelah login (tetap perlu Resume lokal)"; Flags: unchecked
[Icons]
Name: "{group}\RelayDesk"; Filename: "{app}\relaydesk.exe"
Name: "{group}\Uninstall RelayDesk"; Filename: "{uninstallexe}"
Name: "{userstartup}\RelayDesk"; Filename: "{app}\relaydesk.exe"; Parameters: "connector --autostart"; Tasks: autostart
[Run]
Filename: "{app}\relaydesk.exe"; Description: "Buka RelayDesk dan pairing PC"; Flags: postinstall nowait skipifsilent
; No [UninstallDelete]: pairing and unrelated user files must survive uninstall.
