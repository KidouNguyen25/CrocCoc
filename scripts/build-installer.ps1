param(
    [Parameter(Mandatory=$true)][ValidatePattern('^\d+\.\d+\.\d+(-[0-9A-Za-z]+(\.[0-9A-Za-z]+)*)?$')][string]$PackageVersion,
    [string]$IsccPath = "",
    [switch]$DownloadWebView2Bootstrapper,
    [string]$WebView2InstallerPath = "",
    [ValidatePattern("^(|[A-Fa-f0-9]{64})$")][string]$WebView2InstallerSha256 = ""
)
$ErrorActionPreference = "Stop"
if ($env:OS -ne "Windows_NT") { throw "Windows installer build is required." }
$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$release = Join-Path $root "release"
$stage = Join-Path $release "CrocCoc-$PackageVersion-win64"
$manifest = Get-Content -Raw (Join-Path $stage "release-manifest.json") | ConvertFrom-Json
if ($manifest.releaseVersion -ne $PackageVersion -or $manifest.sourceDirty -ne $false -or $manifest.qualityGatesSkipped -ne $false) {
    throw "Not a clean, fully checked candidate of the requested version."
}
$app = (Get-Content -Raw (Join-Path $root "wails.json") | ConvertFrom-Json).info.productVersion
if (($PackageVersion -split '-', 2)[0] -ne $app -or $manifest.appVersion -ne $app) { throw "Version mismatch." }
$head = (& git -C $root rev-parse HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or $manifest.gitSha -ne $head) { throw "Candidate SHA differs from checkout." }
$dirty = & git -C $root status --porcelain
if ($LASTEXITCODE -ne 0 -or $dirty) { throw "Installer requires clean source." }
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
$icon = Join-Path $root "build/windows/icon.ico"
if (-not (Test-Path -LiteralPath $icon -PathType Leaf)) { throw "Wails application icon missing." }
if ($DownloadWebView2Bootstrapper -and $WebView2InstallerPath) { throw "Choose exactly one WebView2 source." }
if ($WebView2InstallerPath -and -not $WebView2InstallerSha256) { throw "Offline WebView2 input requires an expected SHA-256." }
$compilerArgs = @("/DStageDir=$stage", "/DReleaseVersion=$PackageVersion", "/DNumericVersion=$app.0", "/DOutputDir=$release", "/DSetupIconPath=$icon")
$temp = $null
try {
    if ($DownloadWebView2Bootstrapper -or $WebView2InstallerPath) {
        $temp = Join-Path $release (".webview2-" + [guid]::NewGuid().ToString("N"))
        New-Item -ItemType Directory -Path $temp | Out-Null
        $runtime = Join-Path $temp "CrocCoc-WebView2Setup.exe"
        if ($DownloadWebView2Bootstrapper) {
            Invoke-WebRequest -UseBasicParsing -Uri "https://go.microsoft.com/fwlink/p/?LinkId=2124703" -OutFile $runtime
        } else {
            Copy-Item -LiteralPath $WebView2InstallerPath -Destination $runtime
        }
        $signature = Get-AuthenticodeSignature -LiteralPath $runtime
        if ($signature.Status -ne "Valid" -or $signature.SignerCertificate.Subject -notmatch '(?i)(^|,\s*)O=Microsoft Corporation(,|$)') {
            throw "WebView2 prerequisite must carry a trusted Microsoft Authenticode signature."
        }
        $hash = (Get-FileHash -LiteralPath $runtime -Algorithm SHA256).Hash
        if ($WebView2InstallerSha256 -and $hash -ne $WebView2InstallerSha256) { throw "WebView2 SHA-256 mismatch." }
        Write-Output "Embedded WebView2 prerequisite SHA-256: $hash"
        $compilerArgs += @("/DWebView2InstallerPath=$runtime", "/DWebView2InstallerHash=$hash")
    }
    & $IsccPath @compilerArgs (Join-Path $root "installer/CrocCoc.iss")
    if ($LASTEXITCODE -ne 0) { throw "Installer build failed ($LASTEXITCODE)." }
} finally {
    if ($temp -and (Test-Path -LiteralPath $temp)) { Remove-Item -LiteralPath $temp -Recurse -Force }
}
$setup = Join-Path $release "CrocCoc-$PackageVersion-win64-setup.exe"
if (-not (Test-Path -LiteralPath $setup)) { throw "Installer not produced." }
$zip = Join-Path $release "CrocCoc-$PackageVersion-win64.zip"
@($zip, $setup) | ForEach-Object {
    $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $_).Hash
    "$hash  $([IO.Path]::GetFileName($_))"
} | Set-Content -Encoding ascii (Join-Path $release "download-SHA256SUMS.txt")
Get-FileHash -Algorithm SHA256 -LiteralPath $setup
Write-Output "Candidate installer only; clean-machine QA and Authenticode remain separate gates."