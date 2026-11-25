; Inno Setup 6. Stage verified by scripts/build-installer.ps1.
[Setup]
AppId={{8F94D7C2-6B3E-4D1A-9C52-7E3A1B4F8C90}
AppName=CrocCoc
AppVersion={#ReleaseVersion}
VersionInfoVersion={#NumericVersion}
VersionInfoProductVersion={#NumericVersion}
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
SetupIconFile={#SetupIconPath}
UninstallDisplayIcon={app}\CrocCoc.exe
CloseApplications=yes
RestartApplications=no
[Files]
#ifdef WebView2InstallerPath
Source: "{#WebView2InstallerPath}"; DestName: "CrocCoc-WebView2Setup.exe"; Flags: dontcopy noencryption
#endif
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

function HasRuntime(): Boolean;
begin
  Result := HasRuntimeAt(HKLM32) or HasRuntimeAt(HKCU);
end;

function PrepareToInstall(var NeedsRestart: Boolean): String;
var
  InstallerPath: String;
  ExitCode: Integer;
begin
  Result := '';
  if HasRuntime() then exit;
#ifdef WebView2InstallerPath
  try
    ExtractTemporaryFile('CrocCoc-WebView2Setup.exe');
    InstallerPath := ExpandConstant('{tmp}\CrocCoc-WebView2Setup.exe');
    if CompareText(GetSHA256OfFile(InstallerPath), '{#WebView2InstallerHash}') <> 0 then begin
      Result := 'WebView2 installer integrity check failed.';
      exit;
    end;
    if not Exec(InstallerPath, '/silent /install', ExpandConstant('{tmp}'), SW_HIDE, ewWaitUntilTerminated, ExitCode) then begin
      Result := 'Could not start the Microsoft WebView2 installer.';
      exit;
    end;
    Log('WebView2 installer exit code: ' + IntToStr(ExitCode));
    if HasRuntime() then exit;
  except
    Log('WebView2 installer could not be extracted or verified.');
  end;
#endif
  Result := 'Microsoft Edge WebView2 Evergreen Runtime is required. Install the official Evergreen Standalone Installer from https://developer.microsoft.com/en-us/microsoft-edge/webview2/ (offline) or retry online, then run CrocCoc Setup again.';
end;