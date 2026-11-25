# Hosted CI only. Do not install or uninstall on a developer's workstation.
param(
    [Parameter(Mandatory=$true)][ValidatePattern('^\d+\.\d+\.\d+-(rc|mvp|beta)\.[1-9]\d*$')][string]$PackageVersion
)
$ErrorActionPreference = "Stop"
if ($env:GITHUB_ACTIONS -ne "true" -or $env:RUNNER_ENVIRONMENT -ne "github-hosted" -or $env:OS -ne "Windows_NT") {
    throw "Installer smoke is restricted to disposable GitHub-hosted Windows runners."
}
$root = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
$release = Join-Path $root "release"
$name = "CrocCoc-$PackageVersion-win64"
$stage = Join-Path $release $name
$setup = Join-Path $release "$name-setup.exe"
$installDir = Join-Path $env:RUNNER_TEMP ("croccoc-smoke-" + [guid]::NewGuid().ToString("N"))
$uninstaller = Join-Path $installDir "unins000.exe"
$logPath = Join-Path $env:RUNNER_TEMP "croccoc-install-smoke.log"
if (Test-Path -LiteralPath $installDir) { throw "Smoke directory already exists." }
$manifest = Get-Content -Raw -LiteralPath (Join-Path $stage "release-manifest.json") | ConvertFrom-Json
$setupHash = (Get-FileHash -LiteralPath $setup -Algorithm SHA256).Hash
try {
    $proc = Start-Process -FilePath $setup -ArgumentList @("/VERYSILENT", "/SUPPRESSMSGBOXES", "/NORESTART", "/SP-", ('/DIR="' + $installDir + '"'), ('/LOG="' + $logPath + '"')) -Wait -PassThru
    if ($proc.ExitCode -ne 0) { throw "Installer smoke exited $($proc.ExitCode)." }
    foreach ($item in $manifest.artifacts) {
        $actual = Get-FileHash -LiteralPath (Join-Path $installDir $item.path) -Algorithm SHA256
        if ($actual.Hash -ne $item.sha256) { throw "Installed bytes differ: $($item.path)" }
    }
    foreach ($rel in @("SHA256SUMS.txt", "release-manifest.json")) {
        if ((Get-FileHash -LiteralPath (Join-Path $stage $rel)).Hash -ne (Get-FileHash -LiteralPath (Join-Path $installDir $rel)).Hash) {
            throw "Installed metadata differs: $rel"
        }
    }
    if (-not (Test-Path -LiteralPath $uninstaller -PathType Leaf)) { throw "Uninstaller missing." }
} finally {
    if (Test-Path -LiteralPath $uninstaller -PathType Leaf) {
        $proc = Start-Process -FilePath $uninstaller -ArgumentList @("/VERYSILENT", "/SUPPRESSMSGBOXES", "/NORESTART") -Wait -PassThru
        if ($proc.ExitCode -ne 0) { throw "Uninstaller smoke exited $($proc.ExitCode)." }
    }
}
foreach ($rel in @("CrocCoc.exe", "bin/croccoc-engine.exe", "release-manifest.json")) {
    if (Test-Path -LiteralPath (Join-Path $installDir $rel)) { throw "Uninstaller left installed artifact: $rel" }
}
[pscustomobject]@{
    result = "PASS"
    scope = "Hosted Windows runner install/installed bytes/uninstall only; native UI, transfer and missing-runtime tests not covered"
    gitSha = $manifest.gitSha
    installerSha256 = $setupHash
    version = $PackageVersion
    windows = [Environment]::OSVersion.VersionString
} | ConvertTo-Json | Set-Content -Encoding utf8 -LiteralPath (Join-Path $release "installer-smoke.json")
Write-Output "PASS: hosted installer install, installed hashes and uninstall."
