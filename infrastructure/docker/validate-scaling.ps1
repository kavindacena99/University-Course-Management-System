# Reversible local failure exercise. Requires a healthy stack with one Analytics
# replica. Restores one replica in finally; never removes persistent volumes.
param([string]$Project = 'platform-dev', [string]$GatewayUrl = 'http://localhost:8080')
$ErrorActionPreference = 'Stop'
$compose = @('compose', '-f', 'infrastructure/docker/compose.yaml', '-f',
    'infrastructure/docker/compose.dev.yaml', '-p', $Project)
function Invoke-Compose {
    & docker @compose @args
    if ($LASTEXITCODE -ne 0) { throw "Compose failed: $args" }
}
function Send-Probes([string]$Marker, [int]$Count, [switch]$AllowTransient) {
    for ($i = 0; $i -lt $Count; $i++) {
        $request = New-Object System.Net.Http.HttpRequestMessage(
            [System.Net.Http.HttpMethod]::Get, "$GatewayUrl/api/v1/analytics/students/me")
        $request.Headers.Add('X-Request-ID', "$Marker-$i")
        $response = $client.SendAsync($request).GetAwaiter().GetResult()
        try {
            if ($AllowTransient -and [int]$response.StatusCode -in @(502, 504)) {
                Write-Output "OBSERVED transient $([int]$response.StatusCode) during DNS convergence"
            } elseif ([int]$response.StatusCode -ne 401) {
                throw "Probe $Marker-$i returned $([int]$response.StatusCode)"
            }
        } finally { $response.Dispose(); $request.Dispose() }
        Start-Sleep -Milliseconds 100
    }
}
function Assert-Distribution([string]$Marker, [string[]]$Ids) {
    foreach ($id in $Ids) {
        $lines = @(& docker logs $id 2>&1 | Select-String -SimpleMatch $Marker)
        if ($LASTEXITCODE -ne 0 -or $lines.Count -eq 0) {
            throw "No correlated requests reached replica $id"
        }
        Write-Output "PASS replica $id served $($lines.Count) correlated requests ($Marker)"
    }
}
function Wait-Routing {
    # NXDOMAIN caching, passive-failure windows and connection timeouts can
    # outlast one DNS validity interval. Measure bounded recovery, not uptime.
    for ($attempt = 1; $attempt -le 12; $attempt++) {
        $response = $client.GetAsync("$GatewayUrl/api/v1/analytics/students/me").GetAwaiter().GetResult()
        try { $status = [int]$response.StatusCode } finally { $response.Dispose() }
        if ($status -eq 401) { return }
        if ($status -notin @(502, 504)) { throw "Unexpected recovery response $status" }
        Write-Output "OBSERVED recovery attempt $attempt -> $status"
        Start-Sleep -Seconds 5
    }
    throw 'Analytics routing did not recover within twelve attempts'
}
Push-Location (Join-Path $PSScriptRoot '../..')
Add-Type -AssemblyName System.Net.Http
$client = New-Object System.Net.Http.HttpClient
$client.Timeout = [TimeSpan]::FromSeconds(45)
$started = $false
try {
    $initial = @(Invoke-Compose ps -q analytics)
    if ($initial.Count -ne 1) { throw 'Start with exactly one Analytics replica' }
    $gatewayBefore = Invoke-Compose ps -q gateway
    $started = $true
    Invoke-Compose up -d --no-deps --scale analytics=2 --wait analytics
    $ids = @(Invoke-Compose ps -q analytics)
    if ($ids.Count -ne 2) { throw 'Expected two replicas' }
    Start-Sleep -Seconds 15
    Invoke-Compose exec -T gateway nslookup analytics
    $marker = 'scale-' + [guid]::NewGuid().ToString('N')
    Send-Probes $marker 20
    Assert-Distribution $marker $ids

    & docker stop $ids[1]
    if ($LASTEXITCODE -ne 0) { throw 'Stopping replica failed' }
    $marker = 'failover-' + [guid]::NewGuid().ToString('N')
    Send-Probes $marker 4 -AllowTransient
    Start-Sleep -Seconds 15
    Send-Probes $marker 8
    Assert-Distribution $marker @($ids[0])
    Write-Output 'PASS eight requests reached the survivor after DNS convergence'

    # Removing a stopped application container does not remove its database.
    & docker rm $ids[1]
    if ($LASTEXITCODE -ne 0) { throw 'Removing stopped replica failed' }
    Invoke-Compose up -d --no-deps --scale analytics=2 --wait analytics
    $replacementIds = @(Invoke-Compose ps -q analytics)
    if ($replacementIds.Count -ne 2 -or $replacementIds -contains $ids[1]) {
        throw 'Replica was not replaced'
    }
    Start-Sleep -Seconds 15
    $marker = 'replacement-' + [guid]::NewGuid().ToString('N')
    Send-Probes $marker 20
    Assert-Distribution $marker $replacementIds
    if ((Invoke-Compose ps -q gateway) -ne $gatewayBefore) {
        throw 'Gateway unexpectedly recreated'
    }
    Write-Output 'PASS replacement discovered without gateway restart'

    Invoke-Compose stop analytics
    $response = $client.GetAsync("$GatewayUrl/api/v1/analytics/students/me").GetAwaiter().GetResult()
    try {
        $status = [int]$response.StatusCode
        if ($status -notin @(502, 504)) { throw "Expected 502/504 outage, got $status" }
        Write-Output "PASS all replicas unavailable -> $status"
    } finally { $response.Dispose() }
} finally {
    try {
        if ($started) {
            Invoke-Compose up -d --no-deps --scale analytics=1 --wait analytics
            Wait-Routing
            Send-Probes ('recovered-' + [guid]::NewGuid().ToString('N')) 3
            Write-Output 'PASS restored one healthy Analytics replica and gateway routing'
        }
    } finally { $client.Dispose(); Pop-Location }
}
