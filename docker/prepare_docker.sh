#!/bin/bash

if [ ! -f .env ]; then
  cat README.md
  exit 0
fi

if [[ "$OSTYPE" != "darwin"* && "$EUID" -ne 0 ]]; then
  echo "Please run as root or with sudo"
  exit
fi

# Prepare db directory
mkdir -p data
mkdir -p data/grafana
mkdir -p data/prometheus
mkdir -p data/dashboard
if [[ "$OSTYPE" != "darwin"* ]]; then
  chown -R 65532:65532 data
  chown 65532:65532 peering.json
fi
