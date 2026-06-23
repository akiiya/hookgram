param(
    [string]$GoExe = "C:\go_v1.26\bin\go.exe"
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$WebDir = Join-Path $Root "web"
$BuildScript = Join-Path $Root "scripts\build-windows.ps1"
$SmokeScript = Join-Path $Root "scripts\smoke-local.ps1"
$ExePath = Join-Path $Root "dist\hookgram.exe"

function Zh([string]$Base64) {
    [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($Base64))
}

function Say([string]$Base64, [string]$Suffix = "") {
    Write-Host "$(Zh $Base64)$Suffix"
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

try {
    Say "5byA5aeLIEhvb2tncmFtIFJDIOWPkeW4g+mqjOivgQ=="

    Say "5qOA5p+lIEdvIOe8luivkeWZqA=="
    if (!(Test-Path $GoExe)) {
        throw "$(Zh '5pyq5om+5YiwIEdvIOe8luivkeWZqO+8mg==')$GoExe"
    }
    $GoFmt = Join-Path (Split-Path -Parent $GoExe) "gofmt.exe"
    if (!(Test-Path $GoFmt)) {
        throw "$(Zh '5pyq5om+5YiwIEdvIOe8luivkeWZqO+8mg==')$GoFmt"
    }

    Say "5qOA5p+lIE5vZGUvbnBt"
    if (!(Get-Command node.exe -ErrorAction SilentlyContinue)) {
        throw "$(Zh '5pyq5om+5YiwIG5vZGUuZXhl77yM6K+35YWI5a6J6KOFIE5vZGUuanM=')"
    }
    if (!(Get-Command npm.cmd -ErrorAction SilentlyContinue)) {
        throw "$(Zh '5pyq5om+5YiwIG5wbS5jbWTvvIzor7flhYjlronoo4UgTm9kZS5qcw==')"
    }

    Say "5qOA5p+lIHJn"
    $Rg = Get-Command rg -ErrorAction SilentlyContinue
    if ($null -eq $Rg) {
        throw "$(Zh '5pyq5om+5YiwIHJn77yM6K+35YWI5a6J6KOFIHJpcGdyZXAg5oiW5Yqg5YWlIFBBVEg=')"
    }

    $env:GOTELEMETRY = "off"
    $env:GOCACHE = Join-Path $Root ".gocache"
    $env:GOPATH = Join-Path $Root ".gopath"
    $env:npm_config_cache = Join-Path $Root ".npm-cache"
    New-Item -ItemType Directory -Force -Path $env:GOCACHE, $env:GOPATH, $env:npm_config_cache | Out-Null

    Say "5omn6KGMIGdvZm10IOajgOafpQ=="
    $goFiles = Get-ChildItem -Path (Join-Path $Root "cmd"), (Join-Path $Root "internal"), (Join-Path $Root "web") -Recurse -Filter *.go -File
    $gofmtOutput = & $GoFmt -l @($goFiles.FullName)
    if ($LASTEXITCODE -ne 0) {
        throw "gofmt command failed"
    }
    if ($gofmtOutput.Count -gt 0) {
        Say "Z29mbXQg5qOA5p+l5aSx6LSl77yM6K+35YWI5qC85byP5YyW5Lul5LiL5paH5Lu277ya"
        $gofmtOutput | ForEach-Object { Write-Host $_ }
        exit 1
    }

    Say "6L+b5YWlIHdlYiDlronoo4Xkvp3otZY="
    Push-Location $WebDir
    Invoke-Native "npm install failed" "npm.cmd" @("install")

    Say "5p6E5bu65YmN56uv"
    Invoke-Native "npm run build failed" "npm.cmd" @("run", "build")
    Pop-Location

    Say "5omn6KGMIGdvIG1vZCB0aWR5"
    Push-Location $Root
    Invoke-Native "go mod tidy failed" $GoExe @("mod", "tidy")

    Say "5omn6KGMIEdvIOa1i+ivlQ=="
    Invoke-Native "go test failed" $GoExe @("test", "./...")

    Say "5omn6KGM5Y6f55SfIFNRTCDmiavmj48="
    $scanOutput = & $Rg.Source "db\.Raw|db\.Exec|\.Raw\(|\.Exec\(" "." `
        --glob "!web/node_modules/**" `
        --glob "!web/dist/**" `
        --glob "!dist/**" `
        --glob "!data/**" `
        --glob "!.gocache/**" `
        --glob "!.gopath/**" `
        --glob "!.npm-cache/**" `
        --glob "!.smoke/**"
    $scanCode = $LASTEXITCODE
    if ($scanCode -eq 0) {
        $scanOutput | ForEach-Object { Write-Host $_ }
        throw "$(Zh '5Y+R546w55aR5Ly85Y6f55SfIFNRTCDosIPnlKjvvIzor7fmo4Dmn6XkuIrmlrnovpPlh7o=')"
    }
    if ($scanCode -ne 1) {
        throw "rg scan failed with code $scanCode"
    }
    Say "5Y6f55SfIFNRTCDmiavmj4/pgJrov4c="
    Pop-Location

    Say "5omn6KGMIFdpbmRvd3Mg5p6E5bu66ISa5pys"
    Invoke-Native "build-windows failed" "powershell.exe" @("-ExecutionPolicy", "Bypass", "-File", $BuildScript)

    Say "5qOA5p+lIGRpc3QvaG9va2dyYW0uZXhl"
    if (!(Test-Path $ExePath)) {
        throw "$(Zh '5pyq5om+5Yiw5Y+v5omn6KGM5paH5Lu277ya')$ExePath"
    }

    Say "5omn6KGM5pys5ZywIHNtb2tlIHRlc3Q="
    Invoke-Native "smoke-local failed" "powershell.exe" @("-ExecutionPolicy", "Bypass", "-File", $SmokeScript)

    Say "6aqM6K+B6YCa6L+H"
} catch {
    Say "6aqM6K+B5aSx6LSl77ya" "$($_.Exception.Message)"
    exit 1
}
