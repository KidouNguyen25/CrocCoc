param(
    [Parameter(Mandatory=$true)][ValidatePattern('^\d+\.\d+\.\d+(-[0-9A-Za-z]+(\.[0-9A-Za-z]+)*)?$')][string]$PackageVersion,
    [string]$IsccPath = ""
)
$ErrorActionPreference = "Stop"
$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$release = Join-Path $root "release"
$stage = Join-Path $release "CrocCoc-$PackageVersion-win64"
$manifest = Get-Content -Raw (Join-Path $stage "release-manifest.json") | ConvertFrom-Json
if ($manifest.releaseVersion -ne $PackageVersion -or $manifest.sourceDirty -ne $false -or $manifest.qualityGatesSkipped -ne $false) {
    throw "Not a clean, fully checked candidate of the requested version."
}
$head = (& git -C $root rev-parse HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or $manifest.gitSha -ne $head) { throw "Candidate SHA differs from checkout." }
$required = @("CrocCoc.exe", "bin/croccoc-engine.exe", "LICENSE", "THIRD_PARTY_LICENSES.txt", "README.txt", "sbom-go.cdx.json", "sbom-frontend.cdx.json")
foreach ($rel in $required) {
    $entry = @($manifest.artifacts | Where-Object { $_.path -ceq $rel })
    if ($entry.Count -ne 1) { throw "Missing or duplicate manifest entry: $rel" }
    $actual = Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $stage $rel)
    if ($actual.Hash -ne $entry[0].sha256) { throw "Artifact hash mismatch: $rel" }
    if (@(Get-Content -LiteralPath (Join-Path $stage "SHA256SUMS.txt") | Where-Object { $_ -ceq "$($actual.Hash)  $rel" }).Count -ne 1) { throw "Checksum list mismatch: $rel" }
}
$worker = (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $stage "bin/croccoc-engine.exe")).Hash
$pin = [regex]::Match((Get-Content -Raw (Join-Path $root "backend/transfer/worker_hash_windows.go")), 'const PinnedWorkerSHA256 = "([A-F0-9]{64})"').Groups[1].Value
if (-not $pin -or $worker -ne $manifest.workerSha256 -or $worker -ne $pin) { throw "Worker pin mismatch." }
if (-not $IsccPath) {
    $cmd = Get-Command ISCC.exe -ErrorAction SilentlyContinue
    if ($cmd) { $IsccPath = $cmd.Source }
    else { $IsccPath = Join-Path ${env:ProgramFiles(x86)} "Inno Setup 6\ISCC.exe" }
}
if (-not (Test-Path -LiteralPath $IsccPath)) { throw "Inno Setup 6 compiler not found." }
& $IsccPath "/DStageDir=$stage" "/DReleaseVersion=$PackageVersion" "/DOutputDir=$release" (Join-Path $root "installer/CrocCoc.iss")
if ($LASTEXITCODE -ne 0) { throw "Installer build failed ($LASTEXITCODE)." }
$setup = Join-Path $release "CrocCoc-$PackageVersion-win64-setup.exe"
if (-not (Test-Path -LiteralPath $setup)) { throw "Installer not produced." }
Get-FileHash -Algorithm SHA256 -LiteralPath $setup
Write-Output "Candidate installer only; clean-machine QA and Authenticode remain separate gates."
