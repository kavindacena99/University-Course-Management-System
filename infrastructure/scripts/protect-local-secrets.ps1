# Apply NTFS ACLs only to existing credential files and newly generated runtime
# directories. Docker Desktop runs as the current user/System. No firewall changes.
param([Parameter(Mandatory=$true)][ValidateSet('dev','qa','prodlike')][string]$Environment)
$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$identity = [System.Security.Principal.WindowsIdentity]::GetCurrent().Name
if ($Environment -eq 'dev') {
    $targets = @('infrastructure/docker/.env')
    $targets += @('security-and-authentication','user','course','enrollment','assessment','analytics') | ForEach-Object { "services/$_/.env" }
    $targets += 'services/security-and-authentication/var/keys'
} else {
    $directory = if ($Environment -eq 'qa') { 'qa' } else { 'prod' }
    $targets = @("infrastructure/environments/$directory/.env", "infrastructure/environments/$directory/runtime")
}
foreach ($target in $targets) {
    $path = (Resolve-Path -LiteralPath (Join-Path $repo $target)).Path
    if (-not $path.StartsWith($repo + '\', [StringComparison]::OrdinalIgnoreCase)) { throw 'Target outside checkout' }
    $items = @((Get-Item -LiteralPath $path))
    if (Test-Path -LiteralPath $path -PathType Container) {
        $items += @(Get-ChildItem -LiteralPath $path -Recurse -Force)
    }
    foreach ($item in $items) {
        # File ACEs must be direct F entries. Applying inheritable directory
        # flags recursively can leave files without usable access entries.
        $userGrant = if ($item.PSIsContainer) { "${identity}:(OI)(CI)F" } else { "${identity}:F" }
        $systemGrant = if ($item.PSIsContainer) { 'SYSTEM:(OI)(CI)F' } else { 'SYSTEM:F' }
        & icacls $item.FullName '/inheritance:r' '/grant:r' $userGrant '/grant:r' $systemGrant | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "ACL update failed for $target" }
        $acl = Get-Acl -LiteralPath $item.FullName
        if (-not @($acl.Access | Where-Object { $_.IdentityReference.Value -eq $identity -and $_.AccessControlType -eq 'Allow' }).Count) {
            throw "Current-user access missing for $target"
        }
    }
    Write-Output "Protected $target for current user and SYSTEM"
}
