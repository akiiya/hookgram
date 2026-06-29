param(
    [string]$GoExe = "C:\go_v1.26\bin\go.exe",
    [string]$Version = ""
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$WebDir = Join-Path $Root "web"
$DistDir = Join-Path $Root "dist"
$Output = Join-Path $DistDir "hookgram.exe"

function Invoke-Native {
    param(
        [string]$FailMessage,
        [string]$Command,
        [string[]]$Arguments
    )
    & $Command @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw $FailMessage
    }
}

if (!(Test-Path $GoExe)) {
    throw "Go compiler not found: $GoExe"
}

if ([string]::IsNullOrWhiteSpace($Version)) {
    try {
        $GitVersion = (& git -C $Root describe --tags --always --dirty 2>$null)
        if ($LASTEXITCODE -eq 0 -and $GitVersion) {
            $Version = $GitVersion.Trim() -replace '^v', ''
        }
    } catch {
        $Version = ""
    }
}
if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = "dev"
}

if (!(Get-Command node.exe -ErrorAction SilentlyContinue)) {
    throw "node.exe not found. Please install Node.js first."
}
if (!(Get-Command npm.cmd -ErrorAction SilentlyContinue)) {
    throw "npm.cmd not found. Please install Node.js first."
}

$env:GOTELEMETRY = "off"
$env:GOCACHE = Join-Path $Root ".gocache"
$env:GOPATH = Join-Path $Root ".gopath"
$env:npm_config_cache = Join-Path $Root ".npm-cache"
New-Item -ItemType Directory -Force -Path $env:GOCACHE, $env:GOPATH, $env:npm_config_cache | Out-Null

Write-Host "==> Installing frontend dependencies"
$NodeModulesGoMod = Join-Path $WebDir "node_modules\go.mod"
if (Test-Path $NodeModulesGoMod) { Remove-Item -LiteralPath $NodeModulesGoMod -Force }
Push-Location $WebDir
try {
    Invoke-Native "npm install failed" "npm.cmd" @("install")
    Write-Host "==> Building frontend"
    Invoke-Native "npm run build failed" "npm.cmd" @("run", "build")
    [System.IO.File]::WriteAllText((Join-Path $WebDir "node_modules\go.mod"), "module hookgram_node_modules`n`ngo 1.26`n", [System.Text.Encoding]::UTF8)
    New-Item -ItemType File -Force -Path (Join-Path $WebDir "dist\.gitkeep") | Out-Null
} finally {
    Pop-Location
}

Write-Host "==> Building Windows executable"
New-Item -ItemType Directory -Force -Path $DistDir | Out-Null
Push-Location $Root
try {
    $LdFlags = "-s -w -X hookgram/internal/version.Version=$Version"
    Invoke-Native "go build failed" $GoExe @("build", "-trimpath", "-ldflags", $LdFlags, "-o", $Output, "./cmd/server")
} finally {
    Pop-Location
}

Write-Host "==> Built $Output (version=$Version)"