param(
    [Parameter(Mandatory=$true)]
    [ValidateSet('config','preflight','pull','backup','migrate','deploy','health','validate','smoke','stop','status','rollback','sync-config','start-support')]
    [string]$Action,
    [Parameter(Mandatory=$true)]
    [ValidateSet('qa','prodlike')]
    [string]$Environment,
    [Parameter(Mandatory=$true)]
    [string]$Release
)

$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$python = Join-Path $root '.infra-venv\Scripts\python.exe'
if (-not (Test-Path -LiteralPath $python)) {
    throw 'The infrastructure Python environment is missing.'
}
& $python -B (Join-Path $root 'infrastructure\scripts\targetctl.py') $Action --environment $Environment --release $Release
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}
