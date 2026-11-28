param(
    [Parameter(Mandatory=$true)]
    [ValidateSet('init','config','start','migrate','restore','verify','recreate','select-image','stop','backup')]
    [string]$Action,
    [string]$Snapshot,
    [string]$Image,
    [string]$Python = '.infra-venv/Scripts/python.exe'
)
$ErrorActionPreference = 'Stop'
Push-Location (Join-Path $PSScriptRoot '../..')
try {
    $arguments = @('-B','infrastructure/scripts/postgres_rehearsal.py',$Action)
    if ($Snapshot) { $arguments += @('--snapshot',$Snapshot) }
    if ($Image) { $arguments += @('--image',$Image) }
    & $Python @arguments
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
} finally { Pop-Location }
