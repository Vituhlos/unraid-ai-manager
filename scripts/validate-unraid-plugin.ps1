param(
  [string]$Root = "."
)

$ErrorActionPreference = "Stop"

$rootPath = (Resolve-Path -LiteralPath $Root).Path
$version = (Get-Content -Raw -LiteralPath (Join-Path $rootPath "VERSION")).Trim()
$pagePath = Join-Path $rootPath "plugin\src\usr\local\emhttp\plugins\unraid-ai-manager\UnraidAIManager.page"
$phpPath = Join-Path $rootPath "plugin\src\usr\local\emhttp\plugins\unraid-ai-manager\UnraidAIManager.php"
$plgPath = Join-Path $rootPath "dist\unraid-ai-manager.plg"

if ($version -notmatch '^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$') {
  throw "VERSION must be SemVer without leading v. Got: $version"
}

$page = Get-Content -Raw -LiteralPath $pagePath
if ($page -notmatch "(?m)^---\s*$") {
  throw "UnraidAIManager.page must contain a '---' separator between headers and content."
}

if ($page -notmatch [regex]::Escape('require_once "/usr/local/emhttp/plugins/unraid-ai-manager/UnraidAIManager.php";')) {
  throw "UnraidAIManager.page must include UnraidAIManager.php after the separator."
}

if ($page -notmatch "Version=`"$([regex]::Escape($version))`"") {
  throw "UnraidAIManager.page version does not match VERSION ($version)."
}

$php = Get-Content -Raw -LiteralPath $phpPath
if ($php -match 'function\s+(h|cfg_|write_config|generate_key)\b') {
  throw "Plugin PHP contains generic function names. Use uaim_ prefix to avoid Unraid/Dynamix collisions."
}

if (Test-Path -LiteralPath $plgPath) {
  $plg = Get-Content -Raw -LiteralPath $plgPath
  if ($plg -notmatch "<!ENTITY version `"$([regex]::Escape($version))`">") {
    throw "dist/unraid-ai-manager.plg version does not match VERSION ($version)."
  }
  if ($plg -notmatch "/releases/download/v$([regex]::Escape($version))") {
    throw "dist/unraid-ai-manager.plg does not point to release tag v$version."
  }
  if ($plg -notmatch "/etc/rc\.d/rc\.unraid-ai-manager restart") {
    throw "dist/unraid-ai-manager.plg must restart the helper after install/upgrade so old helper processes do not keep running."
  }
}

$txzPath = Join-Path $rootPath "dist\unraid-ai-manager-$version-x86_64-1.txz"
if (Test-Path -LiteralPath $txzPath) {
  $tmp = Join-Path ([System.IO.Path]::GetTempPath()) "unraid-ai-manager-validate-$([System.Guid]::NewGuid().ToString('N'))"
  try {
    New-Item -ItemType Directory -Force -Path $tmp | Out-Null
    tar -xf $txzPath -C $tmp ./etc/rc.d/rc.unraid-ai-manager
    if ($LASTEXITCODE -ne 0) {
      throw "tar failed while inspecting $txzPath"
    }
    $rcPath = Join-Path $tmp "etc\rc.d\rc.unraid-ai-manager"
    $bytes = [System.IO.File]::ReadAllBytes($rcPath)
    if ($bytes.Length -lt 12 -or [System.Text.Encoding]::ASCII.GetString($bytes, 0, 11) -ne "#!/bin/bash") {
      throw "Packaged rc.unraid-ai-manager has an invalid shebang."
    }
    if ($bytes -contains 13) {
      throw "Packaged rc.unraid-ai-manager contains CRLF/CR line endings; Unraid requires LF."
    }
  } finally {
    Remove-Item -LiteralPath $tmp -Recurse -Force -ErrorAction SilentlyContinue
  }
}

"Unraid plugin validation passed for version $version."
