# Ansible CLI container fallback for Windows. It has host-equivalent Docker
# access through the socket. Use only this trusted local checkout/image.
param(
    [Parameter(Mandatory=$true)][ValidateSet('dev','qa','prodlike')][string]$Environment,
    [ValidateSet('deploy','validate','health-check','rollback')][string]$Playbook = 'validate',
    [switch]$SyntaxCheck,
    [ValidateSet('existing','build','pull')][string]$DeploymentMode = 'existing',
    [string]$ReleaseManifest,
    [string]$VaultPasswordFile,
    [string]$Image = 'ucm-ansible-control:local'
)
$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$linuxPath = '/run/desktop/mnt/host/' + $repo.Substring(0,1).ToLower() + '/' + $repo.Substring(3).Replace('\','/')
$ansibleDirectory = "$linuxPath/infrastructure/ansible"
$argsList = @('run','--rm','--mount',"type=bind,source=$repo,target=$linuxPath",
    '--mount','type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock',
    '--workdir',$ansibleDirectory,'--env',"ANSIBLE_CONFIG=$ansibleDirectory/ansible.cfg",
    '--entrypoint','ansible-playbook')
if ($VaultPasswordFile) {
    $passwordPath = (Resolve-Path -LiteralPath $VaultPasswordFile).Path
    if ($passwordPath.StartsWith($repo + '\', [StringComparison]::OrdinalIgnoreCase)) {
        throw 'Vault password files must be outside this repository'
    }
    $argsList += @('--mount', "type=bind,source=$passwordPath,target=/run/vault-password,readonly")
}
$argsList += @($Image,'-i',"inventories/$Environment/hosts.yml", "playbooks/$Playbook.yml", '-e', "deployment_mode=$DeploymentMode")
if ($ReleaseManifest) {
    $releasePath = (Resolve-Path -LiteralPath $ReleaseManifest).Path
    if (-not $releasePath.StartsWith($repo + '\', [StringComparison]::OrdinalIgnoreCase)) {
        throw 'Place the non-secret release manifest inside this checkout for the control-container wrapper'
    }
    $relative = $releasePath.Substring($repo.Length + 1).Replace('\','/')
    $extraVars = @{release_manifest = "$linuxPath/$relative"} | ConvertTo-Json -Compress
    $argsList += @('-e', $extraVars)
}
if ($VaultPasswordFile) { $argsList += @('--vault-password-file','/run/vault-password') }
if ($SyntaxCheck) { $argsList += '--syntax-check' }
& docker @argsList
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
