# zoraxy-prometheus-exporter

A [Zoraxy](https://github.com/tobychui/zoraxy) plugin that exports statistical analysis data as [Prometheus](https://prometheus.io/) metrics.

## Metrics

| Metric | Type | Description |
|---|---|---|
| `zoraxy_requests_today_total` | gauge | Total requests proxied today |
| `zoraxy_requests_today_valid` | gauge | Valid requests today |
| `zoraxy_requests_today_error` | gauge | Error requests today |
| `zoraxy_requests_by_forward_type{type=...}` | gauge | Requests by forward type |
| `zoraxy_requests_by_country{country=...}` | gauge | Requests by origin country ISO code |
| `zoraxy_requests_by_downstream{hostname=...}` | gauge | Requests by downstream hostname |
| `zoraxy_requests_by_upstream{hostname=...}` | gauge | Requests by upstream hostname |
| `zoraxy_network_rx_bits_total` | counter | Accumulated received bits (all interfaces) |
| `zoraxy_network_tx_bits_total` | counter | Accumulated transmitted bits (all interfaces) |
| `zoraxy_stats_last_update_unix` | gauge | Unix timestamp of last successful stats fetch |

Daily metrics (requests) reset at midnight. Network counters reset on system restart.

## Installation

1. Download the binary for your platform from [Releases](https://github.com/reptil1990/zoraxy-prometheus-exporter/releases)
2. Create a folder in your Zoraxy plugins directory with the **same name as the binary**:
   ```
   plugins/
   └── zoraxy-prometheus-exporter/
       └── zoraxy-prometheus-exporter        # Linux binary (no extension)
   ```
3. Make it executable: `chmod +x zoraxy-prometheus-exporter`
4. In the Zoraxy web UI: **Plugins → Reload → Enable**

## Prometheus scrape config

```yaml
scrape_configs:
  - job_name: zoraxy
    static_configs:
      - targets: ['<zoraxy-host>:9100']
```

## Configuration

The metrics port defaults to `9100`. To use a different port, create a `start.sh` in the plugin folder:

```bash
#!/bin/bash
exec "$(dirname "$0")/zoraxy-prometheus-exporter" -metrics-port=9200 "$@"
```

## Building from source

```bash
go build -o zoraxy-prometheus-exporter .
```

## Upstream Repo for plugins

[Repo](https://github.com/aroz-online/zoraxy-official-plugins)
