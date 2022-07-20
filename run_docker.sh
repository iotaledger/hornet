#!/bin/bash

if [[ "$OSTYPE" != "darwin"* && "$EUID" -ne 0 ]]; then
  echo "Please run as root or with sudo"
  exit
fi

# argument must not exist or must start with '-'
if ! [[ $1 = "" || $1 = -* ]]; then
    echo "Usage: ./run_docker.sh [docker compose up options]"
fi

if [[ $1 = "build" ]]; then
  # Build latest code
  docker compose build

  # Pull latest images
  docker compose pull inx-indexer
  docker compose pull inx-mqtt
  docker compose pull inx-participation
  docker compose pull inx-dashboard
fi

# Prepare db directory
mkdir -p testnet
mkdir -p testnet/indexer
mkdir -p testnet/participation
mkdir -p testnet/dashboard
if [[ "$OSTYPE" != "darwin"* ]]; then
  chown -R 65532:65532 testnet
fi

docker compose up $@