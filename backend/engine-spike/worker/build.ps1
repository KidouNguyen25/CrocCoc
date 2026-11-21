$ErrorActionPreference = "Stop"

if ($env:OS -ne "Windows_NT") {
    throw "This script must run on Windows."
}

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..\..")).Path
$patchPath = (Resolve-Path (Join-Path $PSScriptRoot "..\croc-v11.6.4-manifest-only.patch")).Path
$upstreamCommit = "f4cfdc17e06ac8a5834e146666798170138c3391"
$tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd('\')
$buildRoot = Join-Path $tempRoot ("croccoc-engine-spike-" + [guid]::NewGuid().ToString("N"))
$sourcePath = Join-Path $buildRoot "croc"
$modPath = Join-Path $buildRoot "worker.mod"
$sumPath = Join-Path $buildRoot "worker.sum"
$outputPath = Join-Path $PSScriptRoot "croccoc-engine-spike.exe"
$go = (Get-Command go -ErrorAction Stop).Source
$git = (Get-Command git -ErrorAction Stop).Source
$oldLocation = Get-Location
$oldCgo = $env:CGO_ENABLED

function Invoke-Native {
    param([string]$Command, [string[]]$Arguments)
    & $Command @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$([IO.Path]::GetFileName($Command)) failed with exit code $LASTEXITCODE."
    }
}

try {
    New-Item -ItemType Directory -Path $buildRoot | Out-Null
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
    $repoModulePath = $repoRoot.Replace('\', '/')
    $crocModulePath = ([IO.Path]::GetFullPath($sourcePath)).Replace('\', '/')
    Invoke-Native $go @("mod", "edit", "-modfile=$modPath", "-replace=croccoc=$repoModulePath", "-replace=github.com/schollz/croc/v11=$crocModulePath")

    $env:CGO_ENABLED = "0"
    Set-Location $PSScriptRoot
    Invoke-Native $go @("test", "-modfile=$modPath", "-tags", "croc_no_tailcat", "./...")
    Invoke-Native $go @("build", "-modfile=$modPath", "-tags", "croc_no_tailcat", "-o", $outputPath, ".")
    Write-Output "Built pinned Croc worker: $outputPath"
}
finally {
    Set-Location $oldLocation
    $env:CGO_ENABLED = $oldCgo
    $buildRootFull = [IO.Path]::GetFullPath($buildRoot).TrimEnd('\')
    if ([IO.Path]::GetDirectoryName($buildRootFull).TrimEnd('\') -eq $tempRoot -and
        [IO.Path]::GetFileName($buildRootFull).StartsWith("croccoc-engine-spike-", [StringComparison]::Ordinal)) {
        Remove-Item -LiteralPath $buildRootFull -Recurse -Force -ErrorAction SilentlyContinue
    }
}
