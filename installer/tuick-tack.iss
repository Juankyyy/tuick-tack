; Tuick-Tack per-user installer. Build with:
;   scripts\build.ps1 -Version vX.Y.Z -Installer
; The script is intentionally free of machine-specific paths: the build passes
; RepoRoot and AppVersion.

#define AppName "Tuick-Tack"
#define AppExe "tuick-tack.exe"
#define AppPublisher "Juankyyy"
#define AppURL "https://github.com/Juankyyy/tuick-tack"
#define AppId "{{9C3E7A42-6B1D-4F8E-9A2C-7D5B1E4F6A80}"

#ifndef AppVersion
  #define AppVersion "0.0.0"
#endif
#ifndef RepoRoot
  #define RepoRoot ".."
#endif

[Setup]
AppId={#AppId}
AppName={#AppName}
AppVerName={#AppName}
AppVersion={#AppVersion}
AppPublisher={#AppPublisher}
AppPublisherURL={#AppURL}
AppSupportURL={#AppURL}/issues
AppUpdatesURL={#AppURL}/releases
AppCopyright=GPL-3.0
DefaultDirName={userdocs}\tuick-tack
DefaultGroupName={#AppName}
DisableProgramGroupPage=yes
DisableDirPage=auto
AllowNoIcons=yes
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0
OutputDir=..\dist
OutputBaseFilename=tuick-tack-setup
SetupIconFile={#RepoRoot}\assets\icon.ico
UninstallDisplayIcon={app}\{#AppExe}
UninstallDisplayName={#AppName}
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
WizardImageFile={#RepoRoot}\installer\wizard-large.png
WizardSmallImageFile={#RepoRoot}\installer\wizard-small.png
WizardImageStretch=no
CloseApplications=yes
RestartApplications=no
VersionInfoVersion={#AppVersion}.0
VersionInfoProductVersion={#AppVersion}.0
VersionInfoCompany={#AppPublisher}
VersionInfoDescription={#AppName} Setup
VersionInfoProductName={#AppName}
VersionInfoCopyright=GPL-3.0
LicenseFile={#RepoRoot}\LICENSE

[Tasks]
Name: "autostart"; Description: "Start Tuick-Tack with Windows"; GroupDescription: "Startup:"; Flags: unchecked
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "{#RepoRoot}\dist\{#AppExe}"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#RepoRoot}\LICENSE"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\{#AppName}"; Filename: "{app}\{#AppExe}"; WorkingDir: "{app}"; Comment: "Your countdown, in the terminal and in the tray."
Name: "{autodesktop}\{#AppName}"; Filename: "{app}\{#AppExe}"; WorkingDir: "{app}"; Tasks: desktopicon

[Run]
Filename: "{app}\{#AppExe}"; Description: "{cm:LaunchProgram,{#AppName}}"; Flags: nowait postinstall skipifsilent

[UninstallRun]
Filename: "{app}\{#AppExe}"; Parameters: "--uninstall-startup"; Flags: runhidden; RunOnceId: "RemoveAutostart"

[Code]
var
  AutostartWasEnabled: Boolean;
  AutostartTaskFound: Boolean;

function InitializeSetup(): Boolean;
begin
  AutostartWasEnabled := RegValueExists(
    HKEY_CURRENT_USER,
    'Software\Microsoft\Windows\CurrentVersion\Run',
    'TuickTack');
  AutostartTaskFound := False;
  Result := True;
end;

procedure InitializeWizard();
var
  Index: Integer;
begin
  { Preserve an existing autostart setting on an interactive upgrade. The
    silent path uses AutostartWasEnabled directly, and TasksList is empty in
    silent mode, so it must not be touched there. }
  if WizardSilent then
    Exit;
  Index := WizardForm.TasksList.Items.IndexOf('Start Tuick-Tack with Windows');
  AutostartTaskFound := Index >= 0;
  if AutostartTaskFound and AutostartWasEnabled then
    WizardForm.TasksList.Checked[Index] := True;
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
  ResultCode: Integer;
  Enable: Boolean;
begin
  if CurStep <> ssPostInstall then
    Exit;
  if WizardSilent then
    Enable := AutostartWasEnabled
  else
    Enable := WizardIsTaskSelected('autostart') or
      (AutostartWasEnabled and not AutostartTaskFound);
  if Enable then
    Exec(ExpandConstant('{app}\{#AppExe}'), '--install-startup', '',
      SW_HIDE, ewWaitUntilTerminated, ResultCode)
  else
    Exec(ExpandConstant('{app}\{#AppExe}'), '--uninstall-startup', '',
      SW_HIDE, ewWaitUntilTerminated, ResultCode);
end;
