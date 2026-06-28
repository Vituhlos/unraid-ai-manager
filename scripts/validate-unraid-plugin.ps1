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
}

"Unraid plugin validation passed for version $version."
