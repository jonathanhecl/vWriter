Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$projectRoot = $PSScriptRoot
$outputDir = Join-Path $projectRoot "dist"
$executable = Join-Path $outputDir "vWriter.exe"

try {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        throw "Go is not available in PATH. Install Go 1.24 or newer and try again."
    }

    New-Item -ItemType Directory -Path $outputDir -Force | Out-Null
    Push-Location $projectRoot
    try {
        Get-Process -Name "vWriter" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
        Start-Sleep -Milliseconds 200
        Write-Output "[i] Building Windows executable..."
        go build -o $executable .
    }
    finally {
        Pop-Location
    }

    Write-Output "[OK] Built: $executable"
    Write-Output "[i] Launching vWriter..."
    & $executable
}
catch {
    Write-Error "[X] Build or launch failed: $($_.Exception.Message)"
    exit 1
}
