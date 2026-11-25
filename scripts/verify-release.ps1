# Verify the exact staged bytes, portable archive and installer download hashes.
# Candidate mode does not attest installer contents or substitute for native QA.
param(
    [Parameter(Mandatory=$true)][ValidatePattern('^\d+\.\d+\.\d+(-[0-9A-Za-z]+(\.[0-9A-Za-z]+)*)?$')][string]$PackageVersion,
    [switch]$RequireSigned,
    [ValidatePattern('^(|[A-Fa-f0-9]{40})$')][string]$ExpectedSignerThumbprint = "",
    [ValidatePattern('^(|[A-Fa-f0-9]{40})$')][string]$ExpectedWorkerSignerThumbprint = ""
)
$ErrorActionPreference = "Stop"
if ($env:OS -ne "Windows_NT") { throw "Windows verification is required." }
$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$release = Join-Path $root "release"
$name = "CrocCoc-$PackageVersion-win64"
$stage = Join-Path $release $name
$zip = Join-Path $release "$name.zip"
$installer = Join-Path $release "$name-setup.exe"
foreach ($directory in @($stage, (Join-Path $stage "bin"))) {
    $item = Get-Item -LiteralPath $directory
    if (-not $item.PSIsContainer -or ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw "Unsafe staging directory: $directory" }
}
$manifestPath = Join-Path $stage "release-manifest.json"
$manifest = Get-Content -Raw -LiteralPath $manifestPath | ConvertFrom-Json
$app = (Get-Content -Raw -LiteralPath (Join-Path $root "wails.json") | ConvertFrom-Json).info.productVersion
$frontend = Get-Content -Raw -LiteralPath (Join-Path $root "frontend/package.json") | ConvertFrom-Json
$head = (& git -C $root rev-parse HEAD).Trim()
if ($LASTEXITCODE -ne 0) { throw "Cannot resolve Git HEAD." }
$dirty = & git -C $root status --porcelain
if ($LASTEXITCODE -ne 0 -or $dirty) { throw "Verification requires clean reviewed source." }
if ($app -ne $frontend.version -or ($PackageVersion -split '-', 2)[0] -ne $app -or
    $manifest.package -ne $name -or $manifest.releaseVersion -ne $PackageVersion -or $manifest.appVersion -ne $app -or
    $manifest.gitSha -ne $head -or $manifest.sourceDirty -ne $false -or $manifest.qualityGatesSkipped -ne $false) {
    throw "Manifest, version, source or quality gate mismatch."
}
if ($PackageVersion -eq $app -and -not $RequireSigned) { throw "Stable package must pass the signed gate." }
if ($RequireSigned) {
    if ($PackageVersion -ne $app -or -not $ExpectedSignerThumbprint) { throw "Stable verification requires a stable version and the approved signer certificate thumbprint." }
    if (-not $ExpectedWorkerSignerThumbprint) { $ExpectedWorkerSignerThumbprint = $ExpectedSignerThumbprint }
}
function Get-Hash([string]$Path) { (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash }
function Assert-Signed([string]$Path, [string]$Thumbprint) {
    $signature = Get-AuthenticodeSignature -LiteralPath $Path
    if ($signature.Status -ne "Valid" -or $null -eq $signature.SignerCertificate -or
        $signature.SignerCertificate.Thumbprint -ne $Thumbprint -or $null -eq $signature.TimeStamperCertificate) {
        throw "Missing trusted, timestamped Authenticode signature from the approved signer: $Path"
    }
}
$required = @("CrocCoc.exe", "bin/croccoc-engine.exe", "LICENSE", "THIRD_PARTY_LICENSES.txt", "README.txt", "sbom-go.cdx.json", "sbom-frontend.cdx.json")
$all = $required + @("SHA256SUMS.txt", "release-manifest.json")
$expected = @{}
$lengths = @{}
$checksums = @(Get-Content -LiteralPath (Join-Path $stage "SHA256SUMS.txt"))
if (@($manifest.artifacts).Count -ne $required.Count -or $checksums.Count -ne $required.Count) {
    throw "Unexpected manifest or checksum entries."
}
foreach ($rel in $required) {
    $entry = @($manifest.artifacts | Where-Object { $_.path -ceq $rel })
    if ($entry.Count -ne 1) { throw "Missing/duplicate manifest entry: $rel" }
    $file = Get-Item -LiteralPath (Join-Path $stage $rel)
    if ($file.PSIsContainer -or ($file.Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw "Not a regular file: $rel" }
    $hash = Get-Hash $file.FullName
    if ($hash -ne $entry[0].sha256 -or $file.Length -ne $entry[0].bytes -or
        @($checksums | Where-Object { $_ -ceq "$hash  $rel" }).Count -ne 1) { throw "Hash/size mismatch: $rel" }
    $expected[$rel] = $hash
    $lengths[$rel] = $file.Length
}
$pin = [regex]::Match((Get-Content -Raw -LiteralPath (Join-Path $root "backend/transfer/worker_hash_windows.go")), 'const PinnedWorkerSHA256 = "([A-F0-9]{64})"').Groups[1].Value
if (-not $pin -or $expected["bin/croccoc-engine.exe"] -ne $pin -or $manifest.workerSha256 -ne $pin) { throw "Pinned worker mismatch." }
foreach ($rel in @("SHA256SUMS.txt", "release-manifest.json")) {
    $file = Get-Item -LiteralPath (Join-Path $stage $rel)
    if ($file.PSIsContainer -or ($file.Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw "Not a regular file: $rel" }
    $expected[$rel] = Get-Hash $file.FullName
    $lengths[$rel] = $file.Length
}
$stageFiles = @(Get-ChildItem -LiteralPath $stage -Recurse -Force -File)
if ($stageFiles.Count -ne $all.Count) { throw "Unexpected staged files." }
foreach ($file in $stageFiles) {
    $rel = $file.FullName.Substring($stage.Length + 1).Replace("\", "/")
    if ($all -cnotcontains $rel) { throw "Unexpected staged file: $rel" }
}
Add-Type -AssemblyName System.IO.Compression.FileSystem
$archive = [IO.Compression.ZipFile]::OpenRead($zip)
$seen = @{}
$binDirectorySeen = $false
try {
    foreach ($entry in $archive.Entries) {
        $rel = $entry.FullName.Replace("\", "/")
        if ($rel -ceq "bin/" -and $entry.Length -eq 0) {
            if ($binDirectorySeen) { throw "Duplicate ZIP directory entry." }
            $binDirectorySeen = $true
            continue
        }
        if ($all -cnotcontains $rel -or $seen.ContainsKey($rel)) { throw "Unsafe/unexpected/duplicate ZIP entry: $rel" }
        $seen[$rel] = $true
        if ($entry.Length -ne $lengths[$rel]) { throw "ZIP entry length mismatch: $rel" }
        $stream = $entry.Open()
        try { $actual = (Get-FileHash -InputStream $stream -Algorithm SHA256).Hash } finally { $stream.Dispose() }
        if ($actual -ne $expected[$rel]) { throw "ZIP differs from reviewed staging: $rel" }
    }
    if ($seen.Count -ne $all.Count) { throw "Incomplete ZIP." }
} finally { $archive.Dispose() }
foreach ($path in @((Join-Path $stage "CrocCoc.exe"), $installer)) {
    $v = (Get-Item -LiteralPath $path).VersionInfo
    if ("$($v.ProductMajorPart).$($v.ProductMinorPart).$($v.ProductBuildPart)" -ne $app) { throw "Executable product version mismatch: $path" }
}
$downloadChecksums = @(Get-Content -LiteralPath (Join-Path $release "download-SHA256SUMS.txt"))
if ($downloadChecksums.Count -ne 2) { throw "Unexpected download checksum count." }
foreach ($path in @($zip, $installer)) {
    $line = "$(Get-Hash $path)  $([IO.Path]::GetFileName($path))"
    if (@($downloadChecksums | Where-Object { $_ -ceq $line }).Count -ne 1) { throw "Download checksum mismatch: $path" }
}
if ($RequireSigned) {
    Assert-Signed (Join-Path $stage "CrocCoc.exe") $ExpectedSignerThumbprint
    Assert-Signed (Join-Path $stage "bin/croccoc-engine.exe") $ExpectedWorkerSignerThumbprint
    Assert-Signed $installer $ExpectedSignerThumbprint
}
Write-Output "Verified $name (Git $head); signed gate required: $RequireSigned. Installer install/uninstall and E2E QA are separate."