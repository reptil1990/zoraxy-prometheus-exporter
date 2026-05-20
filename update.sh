#!/bin/bash
systemctl stop zoraxy.service
wget -O /opt/zoraxy/plugins/prometheus-exporter/prometheus-exporter https://github.com/reptil1990/zoraxy-prometheus-exporter/releases/download/v1.1.0/prometheus-exporter_linux_amd64
chmod +x /opt/zoraxy/plugins/prometheus-exporter/prometheus-exporter
systemctl start zoraxy.service
