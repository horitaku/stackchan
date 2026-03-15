param(
  [Parameter(Mandatory = $true)]
  [ValidateSet('run', 'kill8080', 'restart', 'restart_bg', 'healthz', 'build_ui')]
  [string]$Action
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$serverDir = Join-Path $repoRoot 'server'
$webuiDir = Join-Path $serverDir 'webui'

function Build-WebUI {
  Write-Output '[server] Building WebUI (Svelte/Vite)'
  Push-Location $webuiDir
  try {
    if (-not (Test-Path (Join-Path $webuiDir 'node_modules'))) {
      npm install
    }
    npm run build
  }
  finally {
    Pop-Location
  }
}

function Stop-Port8080 {
  $procIds = @()
  $conns = Get-NetTCPConnection -LocalPort 8080 -ErrorAction SilentlyContinue
  if ($null -ne $conns) {
    $procIds += $conns | Select-Object -ExpandProperty OwningProcess
  }

  $procIds = $procIds |
    Where-Object { $_ -and $_ -ne 0 } |
    Sort-Object -Unique

  if (($procIds | Measure-Object).Count -eq 0) {
    Write-Output '[server] No process uses port 8080'
    return
  }

  foreach ($procId in $procIds) {
    try {
      Stop-Process -Id $procId -Force -ErrorAction Stop
      Write-Output ("[server] Stopped PID={0}" -f $procId)
    }
    catch {
      Write-Output ("[server] Failed PID={0}: {1}" -f $procId, $_.Exception.Message)
    }
  }
}

function Invoke-Healthz {
  param(
    [string]$Uri = 'http://127.0.0.1:8080/healthz',
    [int]$MaxAttempts = 5,
    [int]$DelayMs = 500
  )

  for ($attempt = 1; $attempt -le $MaxAttempts; $attempt++) {
    try {
      $result = Invoke-RestMethod -Uri $Uri -Method Get -TimeoutSec 5
      Write-Output 'STATUS=200'
      if ($null -ne $result) {
        try {
          $json = $result | ConvertTo-Json -Compress -Depth 4
          Write-Output $json
        }
        catch {
          Write-Output ($result.ToString())
        }
      }
      return
    }
    catch {
      if ($attempt -lt $MaxAttempts) {
        Start-Sleep -Milliseconds $DelayMs
        continue
      }

      Write-Output ('[server] healthz failed after {0} attempts: {1}' -f $MaxAttempts, $_.Exception.Message)
      exit 1
    }
  }
}

switch ($Action) {
  'healthz' {
    Invoke-Healthz
  }
  'kill8080' {
    Stop-Port8080
  }
  'build_ui' {
    Build-WebUI
  }
  'run' {
    Build-WebUI
    Push-Location $serverDir
    try {
      go run ./cmd/stackchan-server
    }
    finally {
      Pop-Location
    }
  }
  'restart' {
    Stop-Port8080
    Build-WebUI
    Push-Location $serverDir
    try {
      go run ./cmd/stackchan-server
    }
    finally {
      Pop-Location
    }
  }
  'restart_bg' {
    Stop-Port8080
    Build-WebUI

    $logDir = Join-Path $repoRoot '.logs'
    if (-not (Test-Path $logDir)) {
      New-Item -ItemType Directory -Path $logDir | Out-Null
    }

    $outLog = Join-Path $logDir 'server.stdout.log'
    $errLog = Join-Path $logDir 'server.stderr.log'

    $proc = Start-Process -FilePath 'go' -ArgumentList 'run', './cmd/stackchan-server' -WorkingDirectory $serverDir -WindowStyle Hidden -RedirectStandardOutput $outLog -RedirectStandardError $errLog -PassThru
    Write-Output ("[server] started in background PID={0}" -f $proc.Id)
    Write-Output ("[server] stdout={0}" -f $outLog)
    Write-Output ("[server] stderr={0}" -f $errLog)
  }
}
