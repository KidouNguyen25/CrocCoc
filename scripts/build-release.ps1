<#
.SYNOPSIS
  Build gói Windows x64 của CrocCoc (CC-07): kiểm thử, build worker đã ghim, build Wails,
  xếp gói, tính SHA-256 và ghi release-manifest.json.
.NOTES
  Chạy tại máy Windows x64 có Go, Node, Wails CLI v2.17.1 và git. Chạy từ bất kỳ thư mục nào.
  Bản build chỉ đáng tin khi nguồn sạch; dùng -AllowDirty cho thử nghiệm cục bộ (ghi rõ trong manifest).
#>
param(
    [switch]$AllowDirty,
    [switch]$SkipTests
)
$ErrorActionPreference = "Stop"

if ($env:OS -ne "Windows_NT") { throw "Script này chỉ chạy trên Windows." }

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $repoRoot

function Invoke-Native {
    param([string]$Command, [string[]]$Arguments)
    & $Command @Arguments
    if ($LASTEXITCODE -ne 0) { throw "$Command $($Arguments -join ' ') thất bại, mã thoát $LASTEXITCODE." }
}
function Get-Sha256([string]$Path) { (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash }

# 1. Điều kiện vào: nguồn sạch, đúng công cụ.
$gitSha = (& git rev-parse HEAD).Trim()
$dirty = (& git status --porcelain)
if ($dirty -and -not $AllowDirty) { throw "Cây làm việc chưa sạch. Commit hoặc dùng -AllowDirty (bản này sẽ không được nghiệm thu)." }

$wailsVersionLine = (& wails version) | Select-Object -Last 1
if ($wailsVersionLine -notmatch "v2\.17\.1") { throw "Cần Wails CLI v2.17.1, đang có: $wailsVersionLine" }
$goVersion = (& go version)
$nodeVersion = (& node --version)

$wailsConfig = Get-Content -Raw (Join-Path $repoRoot "wails.json") | ConvertFrom-Json
$appVersion = $wailsConfig.info.productVersion

# 2. Frontend.
Invoke-Native npm @("--prefix", "frontend", "ci")
if (-not $SkipTests) { Invoke-Native npm @("--prefix", "frontend", "test") }
Invoke-Native npm @("--prefix", "frontend", "run", "build")

# 3. Worker Croc đã ghim (build.ps1 tự dừng nếu hash không khớp backend/transfer/worker_hash_windows.go).
& (Join-Path $repoRoot "backend\engine-spike\worker\build.ps1")
$workerBuilt = Join-Path $repoRoot "backend\engine-spike\worker\croccoc-engine-spike.exe"
if (-not (Test-Path -LiteralPath $workerBuilt)) { throw "Không thấy worker sau khi build." }
$pinText = Get-Content -Raw (Join-Path $repoRoot "backend\transfer\worker_hash_windows.go")
$pin = [regex]::Match($pinText, 'const PinnedWorkerSHA256 = "([A-F0-9]{64})"').Groups[1].Value
if (-not $pin) { throw "Không đọc được hash worker đã ghim." }
if ((Get-Sha256 $workerBuilt) -ne $pin) { throw "Hash worker khác bản ghim." }

# 4. Go test toàn bộ (cần frontend/dist để nhúng).
if (-not $SkipTests) { Invoke-Native go @("test", "-count=1", "./...") }

# 5. Build Wails.
Invoke-Native wails @("build", "-platform", "windows/amd64", "-clean")
$exe = Join-Path $repoRoot "build\bin\CrocCoc.exe"
if (-not (Test-Path -LiteralPath $exe)) { throw "Không thấy build\bin\CrocCoc.exe sau khi build." }

# 6. Xếp gói: CrocCoc.exe cạnh bin\croccoc-engine.exe (main.go tìm worker ở đúng vị trí này).
$packageName = "CrocCoc-$appVersion-win64"
$releaseRoot = Join-Path $repoRoot "release"
$stage = Join-Path $releaseRoot $packageName
if (Test-Path -LiteralPath $stage) { Remove-Item -LiteralPath $stage -Recurse -Force }
New-Item -ItemType Directory -Path (Join-Path $stage "bin") -Force | Out-Null
Copy-Item -LiteralPath $exe -Destination (Join-Path $stage "CrocCoc.exe")
Copy-Item -LiteralPath $workerBuilt -Destination (Join-Path $stage "bin\croccoc-engine.exe")
Copy-Item -LiteralPath (Join-Path $repoRoot "LICENSE") -Destination (Join-Path $stage "LICENSE")
if ((Get-Sha256 (Join-Path $stage "bin\croccoc-engine.exe")) -ne $pin) { throw "Worker trong gói không khớp hash ghim." }

# 7. SHA-256 và manifest.
$files = Get-ChildItem -LiteralPath $stage -Recurse -File | Sort-Object FullName
$sums = foreach ($f in $files) {
    $rel = $f.FullName.Substring($stage.Length + 1).Replace("\", "/")
    [pscustomobject]@{ path = $rel; sha256 = Get-Sha256 $f.FullName; bytes = $f.Length }
}
$sums | ForEach-Object { "$($_.sha256)  $($_.path)" } | Set-Content -Encoding ascii (Join-Path $stage "SHA256SUMS.txt")
[pscustomobject]@{
    package        = $packageName
    appVersion     = $appVersion
    gitSha         = $gitSha
    sourceDirty    = [bool]$dirty
    builtAtUtc     = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
    crocVersion    = "11.6.4"
    crocUpstreamCommit = "f4cfdc17e06ac8a5834e146666798170138c3391"
    workerSha256   = $pin
    tools          = [pscustomobject]@{ go = $goVersion; node = $nodeVersion; wails = $wailsVersionLine }
    artifacts      = $sums
} | ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 (Join-Path $stage "release-manifest.json")

$zip = Join-Path $releaseRoot "$packageName.zip"
if (Test-Path -LiteralPath $zip) { Remove-Item -LiteralPath $zip -Force }
Compress-Archive -Path (Join-Path $stage "*") -DestinationPath $zip
Write-Output "Gói: $zip"
Write-Output "SHA-256 zip: $(Get-Sha256 $zip)"
Write-Output "Chưa nghiệm thu: cần chạy CC-T-11..16 theo docs/e2e-runbook.md trên máy sạch."
