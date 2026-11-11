#Requires -Version 7.0
<#
.SYNOPSIS
  Build the two portable Tuick-Tack executables plus SHA256SUMS.txt.

.DESCRIPTION
  Runs go generate, formatting/vet/tests and builds:
    dist/tuick-tack.exe       console TUI
    dist/tuick-tack-tray.exe  windowsgui daemon (no console)

  Pass -Version v1.0.0 to stamp a release build. The default "dev" keeps
  internal/buildinfo.Version at its development value.
#>
param(
    [string]$Version = "dev"
)

$ErrorActionPreference = "Stop"
$repo = Split-Path -Parent $PSScriptRoot
Set-Location $repo

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

# The .syso files are committed. Regenerate them when the icon or winres files
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
$tuiLdflags = "-s -w -X tuick-tack/internal/buildinfo.Version=$Version"
$trayLdflags = "-s -w -H windowsgui -X tuick-tack/internal/buildinfo.Version=$Version"

go build -trimpath -ldflags $tuiLdflags -o dist/tuick-tack.exe ./cmd/tuick-tack
if ($LASTEXITCODE -ne 0) { throw "TUI build failed" }
go build -trimpath -ldflags $trayLdflags -o dist/tuick-tack-tray.exe ./cmd/tuick-tack-tray
if ($LASTEXITCODE -ne 0) { throw "tray build failed" }

Step "stamping hashes"
$files = @("dist/tuick-tack.exe", "dist/tuick-tack-tray.exe")
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
$trayVersion = (Get-Item .\dist\tuick-tack-tray.exe).VersionInfo.FileVersion
Write-Host "Tuick-Tack tray $trayVersion"
Write-Host ""
Write-Host "Run dist\tuick-tack.exe. It starts the tray daemon on its own."
