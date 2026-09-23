<#
.SYNOPSIS
  gix installer for Windows — downloads the prebuilt binary from GitHub Releases.

.EXAMPLE
  Invoke-WebRequest -Uri "https://raw.githubusercontent.com/m-mdy-m/gix/main/scripts/install.ps1" -OutFile install.ps1
  .\install.ps1

  .\install.ps1 -Version v1.2.3        # specific tag
  .\install.ps1 -Prefix "$env:LOCALAPPDATA\Programs"
#>
[CmdletBinding()]
param(
  [string]$Version = "",     # empty = latest release
  [string]$Prefix  = "$env:LOCALAPPDATA\Programs"
)

$ErrorActionPreference = "Stop"
$Repo   = "m-mdy-m/gix"
$Target = "windows-x64"

function Say($msg) { Write-Host $msg }

# --- pick version ----------------------------------------------------
if (-not $Version) {
  try {
    $rel = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
    $Version = $rel.tag_name
  } catch {
    throw "could not resolve latest release (rate limit? use -Version vX.Y.Z)"
  }
}
$tag  = $Version
$ver  = $Version.TrimStart("v")

# --- download --------------------------------------------------------
$url = "https://github.com/$Repo/releases/download/$tag/gix-$ver-$Target.zip"
$stamp = Get-Date -Format "yyyyMMddHHmmss"
$zip   = Join-Path $env:TEMP "gix-$stamp.zip"
$dest  = Join-Path $env:TEMP "gix-$stamp"

Say "installing gix $tag ($Target)"
Invoke-WebRequest -Uri $url -OutFile $zip -UseBasicParsing

Expand-Archive -Path $zip -DestinationPath $dest -Force
$exe = Get-ChildItem -Path $dest -Filter "gix-*.exe" | Select-Object -First 1
if (-not $exe) { throw "unexpected archive contents" }

# --- install ---------------------------------------------------------
$bindir = Join-Path $Prefix "gix"
New-Item -ItemType Directory -Force -Path $bindir | Out-Null
Copy-Item $exe.FullName -Destination (Join-Path $bindir "gix.exe") -Force

# --- PATH ------------------------------------------------------------
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$bindir*") {
  [Environment]::SetEnvironmentVariable("Path", "$userPath;$bindir", "User")
  Say "added $bindir to your user PATH (restart your terminal)"
}

Remove-Item $zip, $dest -Recurse -Force -ErrorAction SilentlyContinue

Say "installed $bindir\gix.exe ($tag)"
Say "next: gix flow init"
