#!/bin/bash

if [[ "$OSTYPE" != "darwin"* && "$EUID" -ne 0 ]]; then
  echo "Please run as root or with sudo"
  exit
fi

if [[ $1 = "build" ]]; then
  # Build latest code
  docker-compose build

  # Pull latest images
  docker-compose pull inx-indexer
  docker-compose pull inx-mqtt
  docker-compose pull inx-participation
  docker-compose pull inx-dashboard
fi

# Prepare db directory
mkdir -p alphanet
if [[ "$OSTYPE" != "darwin"* ]]; then
  chown -R 65532:65532 alphanet
fi

docker-compose up