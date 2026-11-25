# Parse release entry points without executing builds, installers or network calls.
$ErrorActionPreference = "Stop"
$root = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
$files = @(
    "scripts/build-release.ps1",
    "scripts/build-installer.ps1",
    "scripts/verify-release.ps1",
    "scripts/qa/check-release-scripts.ps1",
    "scripts/qa/smoke-installer.ps1",
    "backend/engine-spike/worker/build.ps1"
)
foreach ($relative in $files) {
    $tokens = $null
    $errors = $null
    $path = Join-Path $root $relative
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "Missing script: $relative" }
    [void][System.Management.Automation.Language.Parser]::ParseFile($path, [ref]$tokens, [ref]$errors)
    if ($errors.Count -gt 0) { throw ("Syntax error in " + $relative + ": " + ($errors | Out-String)) }
    Write-Output "PASS syntax: $relative"
}
