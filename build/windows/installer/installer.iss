#ifndef MyAppVersion
  #define MyAppVersion "0.0.0"
#endif
#ifndef MyAppFilename
  #define MyAppFilename "Predator.exe"
#endif

[Setup]
AppId={{B8C910CC-048E-4764-9653-016B4C5A7B7B}
AppName=Predator
AppVersion={#MyAppVersion}
AppPublisher=Predator
DefaultDirName={autopf}\Predator
DefaultGroupName=Predator
OutputDir=..\..\bin
OutputBaseFilename=Predator-{#MyAppVersion}-windows-setup
Compression=lzma2/ultra64
SolidCompression=yes
SetupIconFile=..\icon.ico
UninstallIconFile=..\icon.ico
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
PrivilegesRequired=lowest
DisableProgramGroupPage=yes
DisableReadyPage=no
WizardStyle=modern
WizardSizePercent=110

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: checkedonce

[Files]
Source: "..\..\bin\{#MyAppFilename}"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\Predator"; Filename: "{app}\{#MyAppFilename}"
Name: "{group}\Uninstall Predator"; Filename: "{uninstallexe}"
Name: "{autodesktop}\Predator"; Filename: "{app}\{#MyAppFilename}"; Tasks: desktopicon

[Run]
Filename: "{app}\{#MyAppFilename}"; Description: "Launch Predator"; Flags: nowait postinstall skipifsilent

[UninstallDelete]
Type: filesandordirs; Name: "{app}"
