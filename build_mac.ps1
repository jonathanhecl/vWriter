Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$projectRoot = $PSScriptRoot
$outputDir = Join-Path $projectRoot "dist"

try {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        throw "Go is not available in PATH. Install Go 1.24 or newer and try again."
    }

    New-Item -ItemType Directory -Path $outputDir -Force | Out-Null
    Push-Location $projectRoot
    try {
        Write-Output "[i] Building macOS (ARM64 & AMD64) executables..."

        $env:GOOS = "darwin"
        $env:GOARCH = "arm64"
        $outputArm64 = Join-Path $outputDir "vWriter_mac_arm64"
        go build -o $outputArm64 .
        Write-Output "[OK] Built macOS Apple Silicon (ARM64): $outputArm64"

        $env:GOARCH = "amd64"
        $outputAmd64 = Join-Path $outputDir "vWriter_mac_amd64"
        go build -o $outputAmd64 .
        Write-Output "[OK] Built macOS Intel (AMD64): $outputAmd64"
    }
    finally {
        Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
        Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue
        Pop-Location
    }
}
catch {
    Write-Error "[X] macOS build failed: $($_.Exception.Message)"
    exit 1
}
