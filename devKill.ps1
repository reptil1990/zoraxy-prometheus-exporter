# Stop the locally running plugin process.
Get-Process -Name "zoraxy-prometheus-exporter" -ErrorAction SilentlyContinue | Stop-Process -Force
Write-Host "Plugin stopped."
