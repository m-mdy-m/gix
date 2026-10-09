<#
.SYNOPSIS
  gix installer for Windows - downloads the prebuilt gix.exe from GitHub
  Releases and puts it on your user PATH. One command:

    irm https://raw.githubusercontent.com/m-mdy-m/gix/main/scripts/install.ps1 | iex

.NOTES
  Options are environment variables, so they work through `| iex`:

    $env:GIX_VERSION = "v1.0.1"                 # a specific tag (default: latest)
    $env:GIX_PREFIX  = "$env:LOCALAPPDATA\Programs"   # gix.exe goes in <prefix>\gix

  Nothing here needs administrator rights.
#>

$ErrorActionPreference = "Stop"
$ProgressPreference    = "SilentlyContinue"   # the progress bar makes downloads slow in Windows PowerShell 5.1

# Windows PowerShell 5.1 does not offer TLS 1.2 by default, and GitHub requires it.
try { [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12 } catch {}

$Repo    = "m-mdy-m/gix"
$Version = $env:GIX_VERSION
$Prefix  = if ($env:GIX_PREFIX) { $env:GIX_PREFIX } else { Join-Path $env:LOCALAPPDATA "Programs" }

function Say($msg) { Write-Host $msg }

# --- platform --------------------------------------------------------
$arch = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
switch ($arch) {
  "AMD64" { $Target = "windows-x64" }
  "ARM64" { $Target = "windows-arm64" }
  default { throw "unsupported architecture: $arch" }
}

# --- pick version ----------------------------------------------------
# The /releases/latest redirect names the tag without the GitHub API,
# so it is not subject to the API's rate limit.
if (-not $Version) {
  try {
    $req = [System.Net.WebRequest]::Create("https://github.com/$Repo/releases/latest")
    $req.AllowAutoRedirect = $false
    $res = $req.GetResponse()
    $loc = $res.Headers["Location"]
    $res.Close()
    $Version = ($loc -split "/")[-1]
  } catch {
    throw "could not work out the latest release - set `$env:GIX_VERSION = 'vX.Y.Z'"
  }
}
if ($Version -notmatch '^v?\d') { throw "could not work out the latest release - set `$env:GIX_VERSION = 'vX.Y.Z'" }
$ver  = $Version.TrimStart("v")
$base = "https://github.com/$Repo/releases/download/v$ver"

# --- download --------------------------------------------------------
$tmp = Join-Path $env:TEMP ("gix-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $tmp | Out-Null

try {
  $exeName = "gix-$Target.exe"
  $exe     = Join-Path $tmp $exeName

  Say "installing gix v$ver ($Target)"

  # Newer releases ship the bare .exe. Older ones only have a
  # .zip, so fall back to that rather than fail.
  try {
    Invoke-WebRequest -Uri "$base/$exeName" -OutFile $exe -UseBasicParsing
  } catch {
    $zip = Join-Path $tmp "gix.zip"
    try {
      Invoke-WebRequest -Uri "$base/gix-$ver-$Target.zip" -OutFile $zip -UseBasicParsing
    } catch {
      throw "download failed: $base/$exeName (does v$ver exist?)"
    }
    Expand-Archive -Path $zip -DestinationPath $tmp -Force
    if (-not (Test-Path $exe)) { throw "unexpected archive contents" }
  }

  # --- verify checksum (release publishes <binary>.sha256) -----------
  try {
    $sumFile = Join-Path $tmp "sum"
    Invoke-WebRequest -Uri "$base/$exeName.sha256" -OutFile $sumFile -UseBasicParsing
    $want = ((Get-Content $sumFile -Raw).Trim() -split "\s+")[0].ToLower()
    $got  = (Get-FileHash -Algorithm SHA256 $exe).Hash.ToLower()
    if ($want -ne $got) { throw "checksum mismatch for $exeName (corrupted download?)" }
  } catch {
    if ($_.Exception.Message -like "checksum mismatch*") { throw }
    # No checksum file published for this release: carry on.
  }

  # --- install -------------------------------------------------------
  $bindir = Join-Path $Prefix "gix"
  New-Item -ItemType Directory -Force -Path $bindir | Out-Null
  $dest = Join-Path $bindir "gix.exe"
  try {
    Copy-Item $exe -Destination $dest -Force
  } catch {
    throw "could not write $dest - is gix running? Close it and try again."
  }
} finally {
  Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
}

# --- PATH ------------------------------------------------------------
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
$entries  = @($userPath -split ";" | Where-Object { $_ })
if ($entries -notcontains $bindir) {
  [Environment]::SetEnvironmentVariable("Path", (($entries + $bindir) -join ";"), "User")
  Say "added $bindir to your user PATH"
}
# Make it work in this window too, without a restart.
if (($env:Path -split ";") -notcontains $bindir) { $env:Path = "$env:Path;$bindir" }

Say "installed $dest (v$ver)"
Say "next: gix flow init"
