#ifndef AppVersion
  #define AppVersion "0.0.0"
#endif

[Setup]
AppName=CloudSave
AppVersion={#AppVersion}
AppPublisher=JoshuaJVB
AppPublisherURL=https://github.com/JoshuaJVB/cloudsaves
DefaultDirName={autopf}\CloudSave
DefaultGroupName=CloudSave
OutputBaseFilename=CloudSave-Setup
OutputDir=.
Compression=lzma
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=lowest

[Files]
Source: "CloudSave.exe"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\CloudSave"; Filename: "{app}\CloudSave.exe"
Name: "{commondesktop}\CloudSave"; Filename: "{app}\CloudSave.exe"; Tasks: desktopicon

[Tasks]
Name: "desktopicon"; Description: "Create a &desktop shortcut"; GroupDescription: "Additional icons:"

[Run]
Filename: "{app}\CloudSave.exe"; Description: "Launch CloudSave"; Flags: nowait postinstall skipifsilent
