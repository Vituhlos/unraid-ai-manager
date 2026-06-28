param(
  [string]$Version = "",
  [string]$ReleaseTag = "",
  [string]$PackageUrl = "",
  [string]$PluginUrl = "https://github.com/Vituhlos/unraid-ai-manager/releases/latest/download/unraid-ai-manager.plg",
  [string]$GoExe = ".\.tools\go\go1.26.4\go\bin\go.exe",
  [string]$OutDir = ".\dist"
)

$ErrorActionPreference = "Stop"

$root = (Resolve-Path ".").Path
$out = Join-Path $root $OutDir
$buildRoot = Join-Path $root ".plugin_build"
$pkgRoot = Join-Path $buildRoot "pkgroot"

if ([string]::IsNullOrWhiteSpace($Version)) {
  $versionPath = Join-Path $root "VERSION"
  if (-not (Test-Path -LiteralPath $versionPath)) {
    throw "Version was not provided and VERSION file was not found."
  }
  $Version = (Get-Content -Raw -LiteralPath $versionPath).Trim()
}

if ($Version -notmatch '^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$') {
  throw "Version must be SemVer, for example 0.1.1. Got: $Version"
}

if ([string]::IsNullOrWhiteSpace($ReleaseTag)) {
  $ReleaseTag = "v$Version"
}

if ([string]::IsNullOrWhiteSpace($PackageUrl)) {
  $PackageUrl = "https://github.com/Vituhlos/unraid-ai-manager/releases/download/$ReleaseTag"
}

$pkgName = "unraid-ai-manager-$Version-x86_64-1"
$txzName = "$pkgName.txz"
$txzPath = Join-Path $out $txzName
$plgPath = Join-Path $out "unraid-ai-manager.plg"

if (-not (Test-Path -LiteralPath $GoExe)) {
  throw "Go executable not found: $GoExe"
}

powershell -ExecutionPolicy Bypass -File (Join-Path $root "scripts\build.ps1") -GoExe $GoExe -OutDir $OutDir | Out-Null

Remove-Item -LiteralPath $buildRoot -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $pkgRoot, $out | Out-Null

Copy-Item -Path (Join-Path $root "plugin\src\*") -Destination $pkgRoot -Recurse -Force

$binDir = Join-Path $pkgRoot "usr\local\bin"
New-Item -ItemType Directory -Force -Path $binDir | Out-Null
Copy-Item -LiteralPath (Join-Path $out "unraid-ai-helper-linux-amd64") -Destination (Join-Path $binDir "unraid-ai-helper") -Force
Copy-Item -LiteralPath (Join-Path $out "unraid-ai-manager-linux-amd64") -Destination (Join-Path $binDir "unraid-ai-manager") -Force

Get-ChildItem -LiteralPath $out -File |
  Where-Object { $_.Name -match '^unraid-ai-manager-.+-x86_64-1\.txz(\.sha256)?$' } |
  Remove-Item -Force

tar -C $pkgRoot -cJf $txzPath .
if ($LASTEXITCODE -ne 0) {
  throw "tar failed while creating $txzPath"
}

$md5 = (Get-FileHash -LiteralPath $txzPath -Algorithm MD5).Hash.ToLowerInvariant()
$sha = (Get-FileHash -LiteralPath $txzPath -Algorithm SHA256).Hash.ToLowerInvariant()
"$sha  $txzName" | Set-Content -LiteralPath "$txzPath.sha256"

$template = Get-Content -Raw -LiteralPath (Join-Path $root "plugin\unraid-ai-manager.plg.template")
$plg = $template.
  Replace("@VERSION@", $Version).
  Replace("@PACKAGE_URL@", $PackageUrl.TrimEnd("/")).
  Replace("@PLUGIN_URL@", $PluginUrl).
  Replace("@MD5@", $md5)
Set-Content -LiteralPath $plgPath -Value $plg -Encoding UTF8

Get-ChildItem -LiteralPath $out -File |
  Where-Object { $_.Name -like "unraid-ai-manager*" -or $_.Name -like "unraid-ai-helper*" } |
  Sort-Object Name |
  Select-Object Name, Length
