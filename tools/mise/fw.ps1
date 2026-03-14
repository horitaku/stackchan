param(
    [Parameter(Mandatory = $true)]
    [ValidateSet('build', 'port', 'upload', 'monitor', 'upmon', 'clean')]
    [string]$Action
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Get-FirmwarePort {
    $lines = py -m platformio device list
    $port = $null

    for ($i = 0; $i -lt $lines.Count; $i++) {
        if ($lines[$i] -match '^COM\d+$') {
            $candidate = $lines[$i].Trim()
            for ($j = $i + 1; $j -lt $lines.Count; $j++) {
                if ($lines[$j] -match '^COM\d+$') { break }
                if ($lines[$j] -match 'VID:PID=303A:1001') {
                    $port = $candidate
                    break
                }
            }
            if ($port) { break }
        }
    }

    if (-not $port) {
        $text = $lines -join [Environment]::NewLine
        $match = [regex]::Match($text, '(?m)^COM\d+$')
        if ($match.Success) {
            $port = $match.Value
        }
    }

    if (-not $port) {
        throw 'No serial port found'
    }

    return $port
}

$repoRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$firmwareDir = Join-Path $repoRoot 'firmware'

switch ($Action) {
    'port' {
        Write-Output (Get-FirmwarePort)
        break
    }

    'build' {
        Push-Location $firmwareDir
        try {
            py -m platformio run -e stackchan_cores3
        }
        finally {
            Pop-Location
        }
        break
    }

    'clean' {
        Push-Location $firmwareDir
        try {
            py -m platformio run -e stackchan_cores3 -t clean
        }
        finally {
            Pop-Location
        }
        break
    }

    'upload' {
        $port = Get-FirmwarePort
        Write-Output ("Uploading to " + $port)
        Push-Location $firmwareDir
        try {
            py -m platformio run -e stackchan_cores3 -t upload --upload-port $port
        }
        finally {
            Pop-Location
        }
        break
    }

    'monitor' {
        $port = Get-FirmwarePort
        Write-Output ("Monitoring " + $port)
        Push-Location $firmwareDir
        try {
            py -m platformio device monitor --baud 115200 --port $port --filter direct
        }
        finally {
            Pop-Location
        }
        break
    }

    'upmon' {
        $port = Get-FirmwarePort
        Write-Output ("Upload+Monitor on " + $port)
        Push-Location $firmwareDir
        try {
            py -m platformio run -e stackchan_cores3 -t upload -t monitor --upload-port $port --monitor-port $port
        }
        finally {
            Pop-Location
        }
        break
    }
}
