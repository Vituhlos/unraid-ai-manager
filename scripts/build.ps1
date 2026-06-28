param(
  [string]$GoExe = ".\.tools\go\go1.26.4\go\bin\go.exe",
  [string]$OutDir = ".\dist",
  [switch]$IncludeWindows
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path -LiteralPath $GoExe)) {
  throw "Go executable not found: $GoExe"
}

New-Item -ItemType Directory -Force -Path $OutDir | Out-Null

$targets = @(
  @{ GOOS = "linux"; GOARCH = "amd64"; EXT = "" }
)

if ($IncludeWindows) {
  $targets += @{ GOOS = "windows"; GOARCH = "amd64"; EXT = ".exe" }
} else {
  Get-ChildItem -LiteralPath $OutDir -File -ErrorAction SilentlyContinue |
    Where-Object { $_.Name -match 'windows-amd64\.exe(\.sha256)?$' } |
    Remove-Item -Force
}

$commands = @(
  @{ Name = "unraid-ai-manager"; Package = "./cmd/unraid-ai-manager" },
  @{ Name = "unraid-ai-helper"; Package = "./cmd/unraid-ai-helper" }
)

foreach ($target in $targets) {
  foreach ($command in $commands) {
    $env:GOOS = $target.GOOS
    $env:GOARCH = $target.GOARCH
    $env:CGO_ENABLED = "0"
    $name = "$($command.Name)-$($target.GOOS)-$($target.GOARCH)$($target.EXT)"
    $output = Join-Path $OutDir $name
    & $GoExe build -trimpath -ldflags="-s -w" -o $output $command.Package
    if ($LASTEXITCODE -ne 0) {
      throw "go build failed for $($command.Name) $($target.GOOS)/$($target.GOARCH)"
    }
    Get-FileHash -LiteralPath $output -Algorithm SHA256 | ForEach-Object {
      "$($_.Hash.ToLowerInvariant())  $name" | Set-Content -LiteralPath "$output.sha256"
    }
  }
}

Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue
Remove-Item Env:\CGO_ENABLED -ErrorAction SilentlyContinue

Get-ChildItem -LiteralPath $OutDir -File | Sort-Object Name | Select-Object Name, Length
