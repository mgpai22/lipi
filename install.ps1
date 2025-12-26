#!/usr/bin/env pwsh
# install.ps1 — download & install the right lipi binary for this machine.
# Usage: .\install.ps1 [-Local] [-Tag vX.Y.Z]
#   env: $env:BIN_DIR (default: ~/.local/bin on Linux/macOS, ~\AppData\Local\Programs on Windows)
#
# Downloads from: https://github.com/mgpai22/lipi/releases

param(
    [Parameter(Mandatory = $false)]
    [switch]$Local,

    [Parameter(Mandatory = $false)]
    [switch]$Help,

    [Parameter(Mandatory = $false)]
    [string]$Tag
)

$ErrorActionPreference = "Stop"

$Repo = "mgpai22/lipi"
$GitHubApi = "https://api.github.com/repos/$Repo/releases"
$GitHubDownload = "https://github.com/$Repo/releases/download"

function Write-Error-Exit {
    param([string]$Message)
    Write-Host "error: $Message" -ForegroundColor Red
    exit 1
}

function Test-Command {
    param([string]$Command)
    $null -ne (Get-Command $Command -ErrorAction SilentlyContinue)
}

# Detect OS
$IsWindowsOS = $IsWindows -or ($PSVersionTable.PSVersion.Major -lt 6)

$defaultBinDir = if ($IsWindowsOS) {
    "$env:USERPROFILE\AppData\Local\Programs"
}
else {
    "$env:HOME/.local/bin"
}

if ($Help) {
    Write-Host @"
Usage: .\install.ps1 [-Local] [-Tag vX.Y.Z]

Downloads the correct archive for this machine and installs to:
  BIN_DIR=$defaultBinDir (or current directory if -Local is used)

Options:
  -Local         Download to current directory instead of installing to PATH
  -Tag vX.Y.Z    Install a specific tag instead of the latest (e.g. v0.1.0)

Env:
  `$env:BIN_DIR    Installation directory override
"@
    exit 0
}

$BinDir = if ($env:BIN_DIR) { $env:BIN_DIR } else { $defaultBinDir }
$LocalInstall = $Local.IsPresent
$TagOverride = $Tag

if ($LocalInstall) {
    $BinDir = "."
}
else {
    if (-not (Test-Path $BinDir)) {
        New-Item -ItemType Directory -Path $BinDir -Force | Out-Null
    }
}

# Detect OS
if ($IsWindowsOS) {
    $os = "windows"
}
elseif ($IsMacOS) {
    $os = "darwin"
}
elseif ($IsLinux) {
    $os = "linux"
}
else {
    Write-Error-Exit "unsupported OS"
}

# Detect architecture
$arch = if ($env:PROCESSOR_ARCHITECTURE) {
    switch ($env:PROCESSOR_ARCHITECTURE) {
        "AMD64" { "amd64" }
        "ARM64" { "arm64" }
        default { Write-Error-Exit "unsupported CPU arch: $env:PROCESSOR_ARCHITECTURE" }
    }
}
else {
    $unameM = uname -m 2>$null
    if ($unameM) {
        switch ($unameM) {
            "x86_64"  { "amd64" }
            "amd64"   { "amd64" }
            "aarch64" { "arm64" }
            "arm64"   { "arm64" }
            default   { Write-Error-Exit "unsupported CPU arch: $unameM" }
        }
    }
    else {
        Write-Error-Exit "unable to determine CPU architecture"
    }
}

# Validate supported platform combinations
$platform = "$os-$arch"
switch ($platform) {
    "linux-amd64"   { }
    "linux-arm64"   { }
    "darwin-amd64"  { }
    "windows-amd64" { }
    "darwin-arm64"  {
        Write-Host "info: darwin-arm64 not available, falling back to darwin-amd64 (Rosetta)"
        $arch = "amd64"
        $platform = "darwin-amd64"
    }
    default { Write-Error-Exit "unsupported platform: $platform" }
}

Write-Host "info: detected platform: $platform"

# Determine tag (latest or override)
if ([string]::IsNullOrWhiteSpace($TagOverride)) {
    Write-Host "info: fetching latest release..."
    try {
        $latestUrl = "$GitHubApi/latest"
        $release = Invoke-RestMethod -Uri $latestUrl -UseBasicParsing
        $tag = $release.tag_name
        if (-not $tag) {
            Write-Error-Exit "could not determine latest version"
        }
        Write-Host "info: latest version: $tag"
    }
    catch {
        Write-Error-Exit "failed to fetch latest release: $_"
    }
}
else {
    $tag = $TagOverride
    Write-Host "info: using tag override: $tag"
}

# Construct asset name
if ($os -eq "windows") {
    $assetFile = "lipi-$os-$arch.zip"
}
else {
    $assetFile = "lipi-$os-$arch.tar.gz"
}

$downloadUrl = "$GitHubDownload/$tag/$assetFile"
Write-Host "info: downloading $downloadUrl"

# Create temp directory
$tmpDir = Join-Path $env:TEMP "lipi-install-$(Get-Random)"
New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null

try {
    $archive = Join-Path $tmpDir $assetFile

    # Download with retry
    $maxRetries = 3
    $retryDelay = 2
    $downloaded = $false

    for ($i = 1; $i -le $maxRetries; $i++) {
        try {
            Invoke-WebRequest -Uri $downloadUrl -OutFile $archive -UseBasicParsing
            $downloaded = $true
            break
        }
        catch {
            if ($i -lt $maxRetries) {
                Write-Host "warning: download attempt $i failed, retrying in ${retryDelay}s..."
                Start-Sleep -Seconds $retryDelay
            }
        }
    }

    if (-not $downloaded) {
        Write-Error-Exit "failed to download: $downloadUrl"
    }

    Write-Host "info: extracting $assetFile"

    # Extract archive
    if ($os -eq "windows") {
        Expand-Archive -Path $archive -DestinationPath $tmpDir -Force
    }
    else {
        if (Test-Command "tar") {
            tar -xzf $archive -C $tmpDir
            if ($LASTEXITCODE -ne 0) {
                Write-Error-Exit "tar extraction failed"
            }
        }
        else {
            Write-Error-Exit "tar command not found (required for .tar.gz extraction)"
        }
    }

    # Find executable
    $binName = if ($IsWindowsOS) { "lipi.exe" } else { "lipi" }
    $binSrc = Get-ChildItem -Path $tmpDir -Recurse -File -Depth 2 |
        Where-Object { $_.Name -eq $binName } |
        Select-Object -First 1

    if (-not $binSrc) {
        Write-Error-Exit "could not locate $binName inside archive"
    }

    $installPath = Join-Path $BinDir $binName

    # Move binary
    Move-Item -Path $binSrc.FullName -Destination $installPath -Force

    # Set executable permissions on Unix-like systems
    if (-not $IsWindowsOS) {
        chmod +x $installPath
    }

    Write-Host "success: installed lipi -> $installPath" -ForegroundColor Green

    # Verify installation
    try {
        $version = & $installPath version 2>$null | Select-Object -First 1
        if ($version) {
            Write-Host "info: $version"
        }
    }
    catch {
        # Ignore version check errors
    }

    # PATH hint (skip for local install)
    if (-not $LocalInstall) {
        $pathSep = if ($IsWindowsOS) { ";" } else { ":" }
        $currentPath = $env:PATH
        if (-not $currentPath.Split($pathSep).Contains($BinDir)) {
            Write-Host ""
            Write-Host "note: $BinDir is not in PATH. Add this to your profile:"
            if ($IsWindowsOS) {
                Write-Host "      `$env:PATH = `"$BinDir;`$env:PATH`""
            }
            else {
                Write-Host "      export PATH=`"$BinDir`:`$PATH`""
            }
        }
    }
}
finally {
    # Cleanup
    Remove-Item -Path $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
}
