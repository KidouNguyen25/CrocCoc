param([switch]$SkipVulnScan)

$ErrorActionPreference = "Stop"

if ($env:OS -ne "Windows_NT") {
    throw "This script must run on Windows."
}

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..\..")).Path
$patchPath = (Resolve-Path (Join-Path $PSScriptRoot "..\croc-v11.6.4-manifest-only.patch")).Path
$upstreamCommit = "f4cfdc17e06ac8a5834e146666798170138c3391"
$tempRoot = Join-Path $repoRoot ".tmp"
$buildName = "croccoc-engine-spike-" + [guid]::NewGuid().ToString("N")
$sourceName = "croccoc-engine-spike-source"
$buildRoot = Join-Path $tempRoot $sourceName
$sourcePath = Join-Path $buildRoot "croc"
$modPath = Join-Path $PSScriptRoot ($buildName + ".mod")
$sumPath = Join-Path $PSScriptRoot ($buildName + ".sum")
$outputPath = Join-Path $PSScriptRoot "croccoc-engine-spike.exe"
$go = (Get-Command go -ErrorAction Stop).Source
$git = (Get-Command git -ErrorAction Stop).Source
$oldLocation = Get-Location
$goEnvironment = @{}
foreach ($name in @("CGO_ENABLED", "GOARCH", "GOAMD64", "GOENV", "GOFLAGS", "GOTOOLCHAIN", "GOWORK", "GOOS")) {
    $goEnvironment[$name] = [Environment]::GetEnvironmentVariable($name)
}
$ownsBuildRoot = $false

function Invoke-Native {
    param([string]$Command, [string[]]$Arguments)
    & $Command @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$([IO.Path]::GetFileName($Command)) failed with exit code $LASTEXITCODE."
    }
}

try {
    if (Test-Path -LiteralPath $buildRoot) {
        throw "Build source path already exists; inspect it before retrying."
    }
    New-Item -ItemType Directory -Path $buildRoot | Out-Null
    $ownsBuildRoot = $true
    Invoke-Native $git @("clone", "--filter=blob:none", "--no-checkout", "https://github.com/schollz/croc.git", $sourcePath)
    Invoke-Native $git @("-C", $sourcePath, "checkout", "--quiet", "--detach", $upstreamCommit)
    $actualCommit = (& $git -C $sourcePath rev-parse HEAD).Trim()
    if ($LASTEXITCODE -ne 0 -or $actualCommit -ne $upstreamCommit) {
        throw "Upstream Croc commit does not match pinned commit."
    }
    Invoke-Native $git @("-C", $sourcePath, "apply", "--check", $patchPath)
    Invoke-Native $git @("-C", $sourcePath, "-c", "core.whitespace=cr-at-eol", "apply", $patchPath)

    Copy-Item (Join-Path $PSScriptRoot "go.mod") $modPath
    Copy-Item (Join-Path $PSScriptRoot "go.sum") $sumPath
    $crocModulePath = "../../../.tmp/$sourceName/croc"
    Invoke-Native $go @("mod", "edit", "-modfile=$modPath", "-replace=github.com/schollz/croc/v11=$crocModulePath")

    $env:CGO_ENABLED = "0"
    $env:GOARCH = "amd64"
    $env:GOAMD64 = "v1"
    $env:GOENV = "off"
    $env:GOFLAGS = ""
    $env:GOTOOLCHAIN = "go1.27.4"
    $env:GOWORK = "off"
    $env:GOOS = "windows"
    Set-Location $PSScriptRoot
    Invoke-Native $go @("test", "-modfile=$modPath", "-trimpath", "-buildvcs=false", "-tags", "croc_no_tailcat", "./...")
    Invoke-Native $go @("build", "-modfile=$modPath", "-trimpath", "-buildvcs=false", "-tags", "croc_no_tailcat", "-o", $outputPath, ".")
    if (-not $SkipVulnScan) {
        # Scan the exact compiled worker executable (including patched Croc and stdlib).
        Invoke-Native govulncheck @("-mode=binary", $outputPath)
    }
    $pinPath = Join-Path $repoRoot "backend\transfer\worker_hash_windows.go"
    $pinMatch = [regex]::Match((Get-Content $pinPath -Raw), 'const PinnedWorkerSHA256 = "([A-F0-9]{64})"')
    if (-not $pinMatch.Success) {
        throw "Pinned worker hash is missing or invalid."
    }
    $actualHash = (Get-FileHash -LiteralPath $outputPath -Algorithm SHA256).Hash
    if ($actualHash -ne $pinMatch.Groups[1].Value) {
        throw "Built worker hash does not match pinned worker hash."
    }
    Write-Output "Built pinned Croc worker: $outputPath"
    Write-Output "SHA-256: $actualHash"
}
finally {
    Set-Location $oldLocation
    foreach ($name in $goEnvironment.Keys) {
        [Environment]::SetEnvironmentVariable($name, $goEnvironment[$name])
    }
    $buildRootFull = [IO.Path]::GetFullPath($buildRoot).TrimEnd('\')
    $tempRootFull = [IO.Path]::GetFullPath($tempRoot).TrimEnd('\')
    if ($ownsBuildRoot -and
        [IO.Path]::GetDirectoryName($buildRootFull).TrimEnd('\') -eq $tempRootFull -and
        [IO.Path]::GetFileName($buildRootFull) -eq $sourceName) {
        Remove-Item -LiteralPath $buildRootFull -Recurse -Force -ErrorAction SilentlyContinue
    }
    Remove-Item -LiteralPath $modPath, $sumPath -Force -ErrorAction SilentlyContinue
}
