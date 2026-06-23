param(
    [string]$GoExe = "C:\go_v1.26\bin\go.exe",
    [string]$Version = ""
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$WebDir = Join-Path $Root "web"
$DistDir = Join-Path $Root "dist"
$Output = Join-Path $DistDir "hookgram.exe"
$VersionFile = Join-Path $Root "VERSION"

function Zh([string]$Base64) {
    [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($Base64))
}

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
    throw "$(Zh '5pyq5om+5YiwIEdvIOe8luivkeWZqO+8mg==')$GoExe"
}

if ([string]::IsNullOrWhiteSpace($Version)) {
    if (!(Test-Path $VersionFile)) {
        throw "VERSION file not found: $VersionFile"
    }
    $Version = (Get-Content -Encoding UTF8 -Path $VersionFile -TotalCount 1).Trim()
}

if ($Version -notmatch '^v[0-9]+\.[0-9]+\.[0-9]+(-rc\.[0-9]+)?$') {
    throw "Invalid VERSION: $Version"
}

if (!(Get-Command node.exe -ErrorAction SilentlyContinue)) {
    throw "$(Zh '5pyq5om+5YiwIG5vZGUuZXhl77yM6K+35YWI5a6J6KOFIE5vZGUuanM=')"
}

if (!(Get-Command npm.cmd -ErrorAction SilentlyContinue)) {
    throw "$(Zh '5pyq5om+5YiwIG5wbS5jbWTvvIzor7flhYjlronoo4UgTm9kZS5qcw==')"
}

$env:GOTELEMETRY = "off"
$env:GOCACHE = Join-Path $Root ".gocache"
$env:GOPATH = Join-Path $Root ".gopath"
$env:npm_config_cache = Join-Path $Root ".npm-cache"
New-Item -ItemType Directory -Force -Path $env:GOCACHE, $env:GOPATH, $env:npm_config_cache | Out-Null

Write-Host "==> $(Zh '5a6J6KOF5YmN56uv5L6d6LWW')"
Push-Location $WebDir
try {
    Invoke-Native "$(Zh '5YmN56uv5L6d6LWW5a6J6KOF5aSx6LSl')" "npm.cmd" @("install")

    Write-Host "==> $(Zh '5p6E5bu65YmN56uv')"
    Invoke-Native "$(Zh '5YmN56uv5p6E5bu65aSx6LSl')" "npm.cmd" @("run", "build")
} finally {
    Pop-Location
}

Write-Host "==> $(Zh '5p6E5bu6IFdpbmRvd3Mg5Y+v5omn6KGM5paH5Lu2')"
New-Item -ItemType Directory -Force -Path $DistDir | Out-Null
Push-Location $Root
try {
    $Commit = "unknown"
    try {
        $GitCommit = (& git -C $Root rev-parse --short HEAD 2>$null)
        if ($LASTEXITCODE -eq 0 -and $GitCommit) {
            $Commit = $GitCommit.Trim()
        }
    } catch {
        $Commit = "unknown"
    }
    $BuildDate = [DateTime]::UtcNow.ToString("yyyy-MM-ddTHH:mm:ssZ")
    $LdFlags = "-s -w -X hookgram/internal/version.Version=$Version -X hookgram/internal/version.Commit=$Commit -X hookgram/internal/version.BuildDate=$BuildDate"
    Invoke-Native "$(Zh 'R28g5p6E5bu65aSx6LSl')" $GoExe @("build", "-trimpath", "-ldflags", $LdFlags, "-o", $Output, "./cmd/server")
} finally {
    Pop-Location
}

Write-Host "==> $(Zh '5p6E5bu65a6M5oiQ77ya')$Output"
