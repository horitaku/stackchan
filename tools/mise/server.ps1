param(
  [Parameter(Mandatory = $true)]
  [ValidateSet('run', 'kill8080', 'restart', 'restart_bg', 'healthz')]
  [string]$Action
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$serverDir = Join-Path $repoRoot 'server'

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

switch ($Action) {
  'healthz' {
    try {
      $r = Invoke-WebRequest -Uri 'http://127.0.0.1:8080/healthz' -UseBasicParsing -TimeoutSec 5
      Write-Host ("STATUS={0}" -f $r.StatusCode)
      if ($null -ne $r.Content -and $r.Content.ToString().Length -gt 0) {
        Write-Host $r.Content
      }
    }
    catch {
      Write-Output ("[server] healthz failed: {0}" -f $_.Exception.Message)
      exit 1
    }
  }
  'kill8080' {
    Stop-Port8080
  }
  'run' {
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
