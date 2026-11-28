param(
    [string]$Project = 'platform-dev',
    [string]$GatewayUrl = 'http://localhost:8080',
    [string]$FrontendUrl = 'http://localhost:3000'
)

$ErrorActionPreference = 'Stop'
$compose = @('compose', '-f', 'infrastructure/docker/compose.yaml', '-f',
    'infrastructure/docker/compose.dev.yaml', '-p', $Project)
Push-Location (Join-Path $PSScriptRoot '../..')
try {
    & docker @compose config --quiet
    if ($LASTEXITCODE -ne 0) { throw 'Compose configuration failed' }
    & docker @compose exec -T gateway nginx -t
    if ($LASTEXITCODE -ne 0) { throw 'Nginx configuration failed' }
    Add-Type -AssemblyName System.Net.Http
    $client = New-Object System.Net.Http.HttpClient
    $client.Timeout = [TimeSpan]::FromSeconds(40)
    try {
        $checks = @(
            @($GatewayUrl, '/health', 200),
            @($GatewayUrl, '/api/v1/auth/me', 401),
            @($GatewayUrl, '/api/v1/users/me', 401),
            @($GatewayUrl, '/api/v1/courses', 401),
            @($GatewayUrl, '/api/v1/enrollments/00000000-0000-0000-0000-000000000001', 401),
            @($GatewayUrl, '/api/v1/assessments/00000000-0000-0000-0000-000000000001', 401),
            @($GatewayUrl, '/api/v1/analytics/students/me', 401),
            @($GatewayUrl, '/api/v1/offerings/00000000-0000-0000-0000-000000000001/enrollments', 401),
            @($GatewayUrl, '/api/v1/offerings/00000000-0000-0000-0000-000000000001/assessments', 401),
            @($GatewayUrl, '/api/v1/offerings/00000000-0000-0000-0000-000000000001/results', 401),
            @($GatewayUrl, '/unknown-infrastructure-check', 404),
            @($GatewayUrl, '/internal/events', 404),
            @($FrontendUrl, '/health', 200),
            @($FrontendUrl, '/', 200),
            @($FrontendUrl, '/courses', 200),
            @($FrontendUrl, '/assets/missing-infrastructure-check.js', 404)
        )
        foreach ($check in $checks) {
            $response = $client.GetAsync($check[0] + $check[1]).GetAwaiter().GetResult()
            try {
                $status = [int]$response.StatusCode
                if ($status -ne $check[2]) {
                    throw "$($check[1]): expected $($check[2]), received $status"
                }
                if (-not $response.Headers.Contains('X-Content-Type-Options')) {
                    throw "$($check[1]): missing security headers"
                }
                Write-Output "PASS $($check[0])$($check[1]) -> $status"
            } finally { $response.Dispose() }
        }
    } finally { $client.Dispose() }
    & docker @compose ps -a
    if ($LASTEXITCODE -ne 0) { throw 'Compose status failed' }
    $states = @(& docker @compose ps -a --format json | ForEach-Object { $_ | ConvertFrom-Json })
    if ($LASTEXITCODE -ne 0 -or $states.Count -lt 22) { throw 'Missing stack components' }
    foreach ($state in $states) {
        if ($state.Service -match '-migrate$|^assessment-minio-init$') {
            if ($state.State -ne 'exited' -or $state.ExitCode -ne 0) {
                throw "Setup job failed: $($state.Service)"
            }
        } elseif ($state.State -ne 'running' -or $state.Health -ne 'healthy') {
            throw "Container not healthy: $($state.Service)"
        }
    }
    Write-Output 'PASS all application components healthy and setup jobs successful'
} finally { Pop-Location }
