#Requires -Version 7.0
<#
.SYNOPSIS
  Build the portable Tuick-Tack executable and, optionally, the Inno Setup
  installer plus SHA256SUMS.txt.

.DESCRIPTION
  Runs go generate, formatting/vet/tests and builds:
    dist/tuick-tack.exe        console TUI and background daemon (--daemon)
    dist/tuick-tack-setup.exe  per-user installer (with -Installer)

  Pass -Version v1.0.0 to stamp a release build. The default "dev" keeps
  internal/buildinfo.Version at its development value. Installer builds need
  a numeric version and Inno Setup 6 (ISCC.exe).
#>
param(
    [string]$Version = "dev",
    [switch]$Installer
)

$ErrorActionPreference = "Stop"
$repo = Split-Path -Parent $PSScriptRoot
Set-Location $repo

$appVersion = $Version.TrimStart("v", "V")
if ($Installer -and $Version -eq "dev") {
    throw "The installer needs a release version, for example -Version v0.3.0."
}
if ($Installer -and $appVersion -notmatch '^\d+\.\d+\.\d+$') {
    throw "Inno Setup needs a numeric version like v0.3.0; got '$Version'."
}

function Step([string]$text) {
    Write-Host ""
    Write-Host "==> $text" -ForegroundColor Cyan
}

Step "checking Go"
$go = Get-Command go -ErrorAction SilentlyContinue
if (-not $go) {
    throw "Go was not found on PATH. Install it from https://go.dev/dl/."
}
$env:CGO_ENABLED = "0"
Write-Host (go version)

Step "generating assets"
go generate ./assets
if ($LASTEXITCODE -ne 0) { throw "asset generation failed" }
go generate ./tools/gen-installer
if ($LASTEXITCODE -ne 0) { throw "installer image generation failed" }

# The .syso file is committed. Regenerate it when the icon or winres files
# change; a failure here must not block the build (SPEC §16).
try {
    go generate ./winres
    if ($LASTEXITCODE -ne 0) { throw "go-winres exited with $LASTEXITCODE" }
} catch {
    Write-Warning "go-winres failed; building without a regenerated executable icon: $_"
}

Step "formatting"
$unformatted = @(gofmt -l .)
if ($unformatted.Count -gt 0) {
    $unformatted | ForEach-Object { Write-Host "  unformatted: $_" -ForegroundColor Red }
    throw "gofmt reported unformatted files"
}

Step "vet"
go vet ./...
if ($LASTEXITCODE -ne 0) { throw "go vet failed" }

Step "tests"
go test ./... -count=1
if ($LASTEXITCODE -ne 0) { throw "go test failed" }

Step "building $Version"
$null = New-Item -ItemType Directory -Force -Path dist
# The old layout shipped a second executable. Remove it so dist/ never mixes
# versions or leaves a confusing stale tray binary.
Remove-Item -Force -ErrorAction SilentlyContinue "dist/tuick-tack-tray.exe", "dist/tuick-tack-tray.exe~"

$ldflags = "-s -w -X tuick-tack/internal/buildinfo.Version=$Version"
go build -trimpath -ldflags $ldflags -o dist/tuick-tack.exe ./cmd/tuick-tack
if ($LASTEXITCODE -ne 0) { throw "build failed" }

if ($Installer) {
    Step "building installer"
    $isccPath = $null
    $iscc = Get-Command ISCC.exe -ErrorAction SilentlyContinue
    if ($iscc) {
        $isccPath = $iscc.Source
    } else {
        $candidates = @(
            "C:\Program Files (x86)\Inno Setup 6\ISCC.exe",
            "C:\Program Files\Inno Setup 6\ISCC.exe",
            "$env:LOCALAPPDATA\Programs\Inno Setup 6\ISCC.exe"
        )
        foreach ($candidate in $candidates) {
            if (Test-Path $candidate) {
                $isccPath = $candidate
                break
            }
        }
    }
    if (-not $isccPath) {
        throw "Inno Setup 6 was not found. Install it with: winget install -e --id JRSoftware.InnoSetup"
    }
    Remove-Item -Force -ErrorAction SilentlyContinue "dist/tuick-tack-setup.exe"
    & $isccPath "/DAppVersion=$appVersion" "/DRepoRoot=$repo" "installer\tuick-tack.iss"
    if ($LASTEXITCODE -ne 0) { throw "Inno Setup failed with $LASTEXITCODE" }
    if (-not (Test-Path "dist/tuick-tack-setup.exe")) {
        throw "Inno Setup did not produce dist/tuick-tack-setup.exe"
    }
}

Step "stamping hashes"
$files = @("dist/tuick-tack.exe")
if ($Installer) { $files += "dist/tuick-tack-setup.exe" }
$hashes = foreach ($file in $files) {
    $hash = (Get-FileHash -Algorithm SHA256 $file).Hash.ToLowerInvariant()
    "$hash  $(Split-Path -Leaf $file)"
}
$hashes | Set-Content -Encoding ascii dist/SHA256SUMS.txt

Write-Host ""
Write-Host "Build complete" -ForegroundColor Green
Get-ChildItem dist | Select-Object Name, Length, LastWriteTime | Format-Table
Write-Host ""
Write-Host "Version:"
& .\dist\tuick-tack.exe --version
Write-Host ""
Write-Host "Run dist\tuick-tack.exe. It starts the background daemon on its own."
if ($Installer) {
    Write-Host "The per-user installer is dist\tuick-tack-setup.exe."
}
