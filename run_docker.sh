#!/bin/bash

if [[ "$OSTYPE" != "darwin"* && "$EUID" -ne 0 ]]; then
  echo "Please run as root or with sudo"
  exit
fi

# Build latest code
docker-compose build

# Pull latest images
docker-compose pull inx-indexer
docker-compose pull inx-mqtt

# Prepare db directory
mkdir -p alphanet
if [[ "$OSTYPE" != "darwin"* ]]; then
  chown -R 65532:65532 alphanet
fi

docker-compose up