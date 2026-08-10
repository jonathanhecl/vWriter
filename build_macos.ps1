Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$projectRoot = $PSScriptRoot
$script = Join-Path $projectRoot "build_mac.ps1"
& $script
