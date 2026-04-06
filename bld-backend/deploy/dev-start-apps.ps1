#Requires -Version 5.1
<#
.SYNOPSIS
  Start bld-backend apps in separate terminal windows (go run).

.DESCRIPTION
  Start infra first: docker compose -f deploy/db-local.yaml up -d

.PARAMETER NoExchange
  Skip exchange matcher.

.PARAMETER NoMarketWs
  Skip market-ws.

.PARAMETER NoChainMonitor
  Skip chain-monitor.

.PARAMETER Shell
  pwsh (default) or powershell.
#>
param(
    [switch] $NoExchange,
    [switch] $NoMarketWs,
    [switch] $NoChainMonitor,
    [ValidateSet('pwsh', 'powershell')]
    [string] $Shell = 'pwsh'
)

$ErrorActionPreference = 'Stop'
$BackendRoot = Split-Path -Parent $PSScriptRoot
if (-not (Test-Path (Join-Path $BackendRoot 'go.mod'))) {
    Write-Error "go.mod not found. Run from bld-backend repo: $BackendRoot"
}

$shellExe = Get-Command $Shell -ErrorAction SilentlyContinue
if (-not $shellExe) {
    if ($Shell -eq 'pwsh') {
        Write-Warning 'pwsh not found, using Windows PowerShell.'
        $Shell = 'powershell'
        $shellExe = Get-Command powershell -ErrorAction Stop
    } else {
        throw "Executable not found: $Shell"
    }
}

function Start-BackendApp {
    param(
        [Parameter(Mandatory)][string] $Title,
        [Parameter(Mandatory)][string] $PackageDir
    )
    $runner = Join-Path $PSScriptRoot 'dev-run-one.ps1'
    $spawnArgs = @(
        '-NoExit',
        '-NoLogo',
        '-File',
        $runner,
        '-BackendRoot',
        $BackendRoot,
        '-Title',
        $Title,
        '-PackageDir',
        $PackageDir
    )
    Start-Process -FilePath $shellExe.Source -ArgumentList $spawnArgs -WorkingDirectory $BackendRoot | Out-Null
    Write-Host "Started: $Title (apps/$PackageDir)"
}

Write-Host "Backend root: $BackendRoot" -ForegroundColor Green
Write-Host "Spawning windows...`n" -ForegroundColor Gray

Start-BackendApp 'walletapi' 'walletapi'
Start-Sleep -Milliseconds 400
Start-BackendApp 'userapi' 'userapi'
Start-Sleep -Milliseconds 400
Start-BackendApp 'ordersapi' 'ordersapi'
Start-Sleep -Milliseconds 400
Start-BackendApp 'gatewayapi' 'gatewayapi'
Start-Sleep -Milliseconds 400

if (-not $NoMarketWs) {
    Start-BackendApp 'market-ws' 'market-ws'
    Start-Sleep -Milliseconds 400
}

if (-not $NoExchange) {
    Start-BackendApp 'exchange' 'exchange'
    Start-Sleep -Milliseconds 400
}

if (-not $NoChainMonitor) {
    Start-BackendApp 'chain-monitor' 'chain-monitor'
}

Write-Host "`nDone. Close each window to stop that service." -ForegroundColor Yellow
Write-Host "Infra: docker compose -f deploy/db-local.yaml up -d`n" -ForegroundColor Yellow
