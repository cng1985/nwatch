# build-platforms.ps1：前端依赖安装/构建、Go 分平台编译、版本信息、发布打包。
# 从脚本自身位置定位根目录，不依赖调用者工作目录；任何阶段失败返回非零。
param(
    [string[]]$Targets = @("windows/amd64", "linux/amd64", "linux/arm64"),
    [string]$Version = "",
    [switch]$SkipWeb
)

$ErrorActionPreference = "Stop"
$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $RepoRoot

if (-not $Version) {
    $Version = (Get-Content (Join-Path $RepoRoot "internal/platform/version/version.go") |
        Select-String -Pattern 'var Version = "([^"]+)"').Matches.Groups[1].Value
    if (-not $Version) { $Version = "dev" }
}
$Commit = ""
try { $Commit = (git rev-parse --short HEAD 2>$null) } catch { }
if (-not $Commit) { $Commit = "unknown" }
$BuildTime = (Get-Date -Format "yyyy-MM-ddTHH:mm:ssK")
$LdFlags = "-s -w -X github.com/cng1985/nwatch/internal/platform/version.Version=$Version"

if (-not $SkipWeb) {
    Write-Host "== 前端构建 =="
    Push-Location (Join-Path $RepoRoot "web")
    try {
        if (Test-Path node_modules) {
            npm ci --no-audit --no-fund
            if ($LASTEXITCODE -ne 0) { throw "npm ci 失败" }
        } else {
            npm install --no-audit --no-fund
            if ($LASTEXITCODE -ne 0) { throw "npm install 失败" }
        }
        npm run build
        if ($LASTEXITCODE -ne 0) { throw "前端构建失败" }
    } finally {
        Pop-Location
    }
} else {
    if (-not (Test-Path (Join-Path $RepoRoot "web/dist/index.html"))) {
        throw "web/dist 不存在：请先构建前端（去掉 -SkipWeb）"
    }
}

$ReleaseRoot = Join-Path $RepoRoot "release"
if (Test-Path $ReleaseRoot) { Remove-Item $ReleaseRoot -Recurse -Force }
New-Item -ItemType Directory -Path $ReleaseRoot | Out-Null

foreach ($Target in $Targets) {
    Write-Host "== 编译 $Target =="
    $Parts = $Target -split "/"
    $GOOS = $Parts[0]
    $GOARCH = $Parts[1]
    $Ext = ""
    if ($GOOS -eq "windows") { $Ext = ".exe" }
    $Name = "nwatch-$Version-$GOOS-$GOARCH$Ext"
    $Env:GOOS = $GOOS
    $Env:GOARCH = $GOARCH
    $Env:CGO_ENABLED = "0"
    go build -trimpath -ldflags $LdFlags -o (Join-Path $ReleaseRoot $Name) ./cmd/server
    if ($LASTEXITCODE -ne 0) { throw "Go 编译失败：$Target" }

    $PkgDir = Join-Path $ReleaseRoot "nwatch-$GOOS-$GOARCH"
    New-Item -ItemType Directory -Path $PkgDir | Out-Null
    Move-Item (Join-Path $ReleaseRoot $Name) (Join-Path $PkgDir ("nwatch" + $Ext))
    Copy-Item -Recurse (Join-Path $RepoRoot "configs") (Join-Path $PkgDir "configs")
    if ($GOOS -eq "linux") {
        Copy-Item (Join-Path $RepoRoot "scripts/install-linux.sh") $PkgDir
    }
    $Notes = "版本：$Version`n提交：$Commit`n构建时间：$BuildTime`n目标：$Target`n"
    Set-Content -Path (Join-Path $PkgDir "RELEASE.txt") -Value $Notes
    Write-Host "已输出 $PkgDir"
}

Write-Host "完成：$ReleaseRoot"
exit 0
