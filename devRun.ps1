# Build and run the plugin locally for development.
# Simulates the -configure flag that Zoraxy would pass on startup.
# Adjust ZoraxyPort to match your local Zoraxy management port.

$Port = 5900
$ZoraxyPort = 8000
$ApiKey = "dev-key-not-valid"

$configure = @{
    port          = $Port
    runtime_const = @{
        zoraxy_version   = "dev"
        zoraxy_uuid      = "00000000-0000-0000-0000-000000000000"
        development_build = $true
    }
    api_key     = $ApiKey
    zoraxy_port = $ZoraxyPort
} | ConvertTo-Json -Compress

Write-Host "Building..."
go build -o zoraxy-prometheus-exporter.exe .
if ($LASTEXITCODE -ne 0) { exit 1 }

Write-Host "Starting plugin on port $Port (metrics on :9100)..."
.\zoraxy-prometheus-exporter.exe "-configure=$configure"
