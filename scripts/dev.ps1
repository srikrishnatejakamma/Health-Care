param([switch]$ConfigureOnly)
$ErrorActionPreference = 'Stop'
Set-Location -LiteralPath (Split-Path -Parent $PSScriptRoot)
if (-not (Test-Path -LiteralPath '.env')) {
    $taskSecretBytes = New-Object byte[] 48
    $taskGenerator = [System.Security.Cryptography.RandomNumberGenerator]::Create()
    try { $taskGenerator.GetBytes($taskSecretBytes) } finally { $taskGenerator.Dispose() }
    $taskSecret = [Convert]::ToBase64String($taskSecretBytes)
    $taskEnvironment = (Get-Content -LiteralPath '.env.example' -Raw).Replace('replace-with-a-random-secret-at-least-32-bytes-long', $taskSecret)
    [System.IO.File]::WriteAllText((Join-Path (Get-Location) '.env'), $taskEnvironment, (New-Object System.Text.UTF8Encoding($false)))
    Write-Host 'Created .env with a random development signing key.'
}
if ($ConfigureOnly) { return }
go run ./cmd/api
if ($LASTEXITCODE -ne 0) { throw 'API failed.' }
