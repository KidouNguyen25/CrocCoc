; Inno Setup 6. Stage verified by scripts/build-installer.ps1.
[Setup]
AppId={{8F94D7C2-6B3E-4D1A-9C52-7E3A1B4F8C90}
AppName=CrocCoc
AppVersion={#ReleaseVersion}
AppPublisher=CrocCoc
DefaultDirName={localappdata}\Programs\CrocCoc
DefaultGroupName=CrocCoc
PrivilegesRequired=lowest
ArchitecturesAllowed=x64os
ArchitecturesInstallIn64BitMode=x64os
MinVersion=10.0
OutputDir={#OutputDir}
OutputBaseFilename=CrocCoc-{#ReleaseVersion}-win64-setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
UninstallDisplayIcon={app}\CrocCoc.exe
CloseApplications=yes
RestartApplications=no
[Files]
Source: "{#StageDir}\CrocCoc.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#StageDir}\bin\croccoc-engine.exe"; DestDir: "{app}\bin"; Flags: ignoreversion
Source: "{#StageDir}\LICENSE"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#StageDir}\THIRD_PARTY_LICENSES.txt"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#StageDir}\README.txt"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#StageDir}\sbom-go.cdx.json"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#StageDir}\sbom-frontend.cdx.json"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#StageDir}\SHA256SUMS.txt"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#StageDir}\release-manifest.json"; DestDir: "{app}"; Flags: ignoreversion
[Icons]
Name: "{autoprograms}\CrocCoc"; Filename: "{app}\CrocCoc.exe"; WorkingDir: "{app}"
[Code]
const
  RuntimeKey = 'SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}';

function HasRuntimeAt(Root: Integer): Boolean;
var
  Version: String;
begin
  Result := RegQueryStringValue(Root, RuntimeKey, 'pv', Version);
  if Result then
    Result := (Version <> '') and (Version <> '0.0.0.0');
end;

function InitializeSetup(): Boolean;
var
  ErrorCode: Integer;
begin
  Result := HasRuntimeAt(HKLM32) or HasRuntimeAt(HKCU32);
  if not Result then begin
    Log('WebView2 Evergreen Runtime missing. Install it from Microsoft and retry.');
    if not WizardSilent then begin
      if MsgBox('Microsoft Edge WebView2 Evergreen Runtime is required. Install the official Evergreen Bootstrapper (online) or Standalone Installer (offline), then retry. Open Microsoft download page?', mbConfirmation, MB_YESNO) = IDYES then
        ShellExec('open', 'https://developer.microsoft.com/en-us/microsoft-edge/webview2/', '', '', SW_SHOWNORMAL, ewNoWait, ErrorCode);
    end;
  end;
end;
