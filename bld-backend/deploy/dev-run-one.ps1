#Requires -Version 5.1
param(
    [Parameter(Mandatory)][string] $BackendRoot,
    [Parameter(Mandatory)][string] $Title,
    [Parameter(Mandatory)][string] $PackageDir
)

$winTitle = $Title
try { [Console]::Title = $winTitle } catch { }
try {
    if ($Host.UI -and $Host.UI.RawUI) {
        $Host.UI.RawUI.WindowTitle = $winTitle
    }
} catch { }

Set-Location -LiteralPath $BackendRoot
Write-Host "=== $Title ===" -ForegroundColor Cyan
$pkgPath = Join-Path (Join-Path $BackendRoot 'apps') $PackageDir
go run $pkgPath
