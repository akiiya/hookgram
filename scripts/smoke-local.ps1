param(
    [string]$ExePath = "",
    [int]$Port = 8787
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
if ($ExePath -eq "") {
    $ExePath = Join-Path $Root "dist\hookgram.exe"
}
$BaseUrl = "http://127.0.0.1:$Port"
$DataPath = Join-Path $Root "data"
$BackupPath = $null
$Process = $null

function Zh([string]$Base64) {
    [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($Base64))
}

function Say([string]$Base64, [string]$Suffix = "") {
    Write-Host "$(Zh $Base64)$Suffix"
}

function New-HiddenProcess([string]$FilePath, [string]$WorkingDirectory) {
    $psi = [System.Diagnostics.ProcessStartInfo]::new()
    $psi.FileName = $FilePath
    $psi.WorkingDirectory = $WorkingDirectory
    $psi.UseShellExecute = $false
    $psi.CreateNoWindow = $true
    return [System.Diagnostics.Process]::Start($psi)
}

function Stop-HookgramProcess {
    param($Target)
    if ($null -ne $Target -and -not $Target.HasExited) {
        $Target.Kill()
        $Target.WaitForExit(5000) | Out-Null
        Say "5bey5YGc5q2iIEhvb2tncmFtIOi/m+eoiw=="
    }
}

function Remove-SmokeData {
    if (Test-Path $DataPath) {
        for ($i = 0; $i -lt 10; $i++) {
            try {
                Remove-Item -LiteralPath $DataPath -Recurse -Force
                Say "5bey56e76ZmkIHNtb2tlIOS4tOaXtiBkYXRhIOebruW9lQ=="
                return
            } catch {
                Start-Sleep -Milliseconds 500
            }
        }
        throw "failed to remove smoke data directory"
    }
}

function Assert-Web {
    param(
        [string]$Url,
        [string]$Contains,
        [string]$OkMessage
    )
    $response = Invoke-WebRequest -UseBasicParsing -Uri $Url -TimeoutSec 5
    if ($response.StatusCode -lt 200 -or $response.StatusCode -ge 300) {
        throw "$(Zh '5o6l5Y+j54q25oCB56CB5byC5bi477ya')$($response.StatusCode)"
    }
    if ($Contains -ne "" -and -not ($response.Content -match [regex]::Escape($Contains))) {
        throw "$(Zh '5ZON5bqU5YaF5a655byC5bi477ya')$Url"
    }
    Say $OkMessage $response.StatusCode
    return $response
}

$Failed = $false
$FailureMessage = ""

try {
    Say "5byA5aeLIEhvb2tncmFtIOacrOWcsCBzbW9rZSB0ZXN0"
    if (!(Test-Path $ExePath)) {
        throw "$(Zh '5pyq5om+5Yiw5Y+v5omn6KGM5paH5Lu277ya')$ExePath"
    }

    if (Test-Path $DataPath) {
        $stamp = Get-Date -Format "yyyyMMddHHmmss"
        $BackupPath = Join-Path $Root "data.smoke-backup-$stamp"
        $index = 0
        while (Test-Path $BackupPath) {
            $index++
            $BackupPath = Join-Path $Root "data.smoke-backup-$stamp-$index"
        }
        Rename-Item -LiteralPath $DataPath -NewName (Split-Path -Leaf $BackupPath)
        Say "5qOA5rWL5YiwIGRhdGEg55uu5b2V77yM5bey5Li05pe25aSH5Lu95Li677ya" $BackupPath
    }

    Say "5ZCv5YqoIEhvb2tncmFt77ya" $ExePath
    $Process = New-HiddenProcess $ExePath $Root
    Say "562J5b6F5pyN5Yqh5ZCv5Yqo"

    $started = $false
    for ($i = 0; $i -lt 40; $i++) {
        if ($Process.HasExited) {
            throw "hookgram exited early with code $($Process.ExitCode)"
        }
        try {
            Assert-Web "$BaseUrl/api/setup/status" "initialized" "5qOA5p+lIC9hcGkvc2V0dXAvc3RhdHVzIOmAmui/h++8mg==" | Out-Null
            $started = $true
            break
        } catch {
            Start-Sleep -Milliseconds 500
        }
    }
    if (!$started) {
        throw "$(Zh '5pyN5Yqh5ZCv5Yqo5aSx6LSl')"
    }

    Assert-Web "$BaseUrl/setup" "Hookgram" "5qOA5p+lIC9zZXR1cCDpobXpnaLpgJrov4c=" | Out-Null
    Assert-Web "$BaseUrl/" "Hookgram" "5qOA5p+l6aaW6aG16YCa6L+H" | Out-Null
} catch {
    $Failed = $true
    $FailureMessage = "$($_.Exception.Message)"
} finally {
    $cleanupErrors = @()
    try {
        Stop-HookgramProcess $Process
    } catch {
        $cleanupErrors += "$($_.Exception.Message)"
    }
    try {
        Remove-SmokeData
    } catch {
        $cleanupErrors += "$($_.Exception.Message)"
    }
    if ($null -ne $BackupPath -and (Test-Path $BackupPath)) {
        try {
            Rename-Item -LiteralPath $BackupPath -NewName "data"
            Say "5bey5oGi5aSN5Y6fIGRhdGEg55uu5b2V"
        } catch {
            $cleanupErrors += "$($_.Exception.Message)"
        }
    }
    if ($cleanupErrors.Count -gt 0) {
        $Failed = $true
        $FailureMessage = "$FailureMessage $($cleanupErrors -join '; ')".Trim()
    }
}

if ($Failed) {
    Say "c21va2UgdGVzdCDlpLHotKXvvJo=" $FailureMessage
    exit 1
}

Say "c21va2UgdGVzdCDpgJrov4c="
