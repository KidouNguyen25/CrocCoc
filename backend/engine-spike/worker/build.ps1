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
$oldCgo = $env:CGO_ENABLED
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
    Set-Location $PSScriptRoot
    Invoke-Native $go @("test", "-modfile=$modPath", "-trimpath", "-buildvcs=false", "-tags", "croc_no_tailcat", "./...")
    Invoke-Native $go @("build", "-modfile=$modPath", "-trimpath", "-buildvcs=false", "-tags", "croc_no_tailcat", "-o", $outputPath, ".")
    Write-Output "Built pinned Croc worker: $outputPath"
}
finally {
    Set-Location $oldLocation
    $env:CGO_ENABLED = $oldCgo
    $buildRootFull = [IO.Path]::GetFullPath($buildRoot).TrimEnd('\')
    $tempRootFull = [IO.Path]::GetFullPath($tempRoot).TrimEnd('\')
    if ($ownsBuildRoot -and
        [IO.Path]::GetDirectoryName($buildRootFull).TrimEnd('\') -eq $tempRootFull -and
        [IO.Path]::GetFileName($buildRootFull) -eq $sourceName) {
        Remove-Item -LiteralPath $buildRootFull -Recurse -Force -ErrorAction SilentlyContinue
    }
    Remove-Item -LiteralPath $modPath, $sumPath -Force -ErrorAction SilentlyContinue
}
