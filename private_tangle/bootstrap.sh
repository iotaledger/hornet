#!/bin/bash

if [[ "$OSTYPE" != "darwin"* && "$EUID" -ne 0 ]]; then
  echo "Please run as root or with sudo"
  exit
fi

# Cleanup if necessary
if [ -d "privatedb" ]; then
  ./cleanup.sh
fi

if [[ $1 = "build" ]]; then
  # Build latest code
  docker-compose --profile "bootstrap" build

  # Pull latest images
  docker-compose pull inx-coordinator
  docker-compose pull inx-indexer
  docker-compose pull inx-mqtt
fi

# Create snapshot
mkdir -p snapshots/coo
if [[ "$OSTYPE" != "darwin"* ]]; then
  chown -R 65532:65532 snapshots
fi
docker-compose --profile "snapshots" up

# Duplicate snapshot for all nodes
cp -R snapshots/coo snapshots/hornet-2
cp -R snapshots/coo snapshots/hornet-3
cp -R snapshots/coo snapshots/hornet-4
if [[ "$OSTYPE" != "darwin"* ]]; then
  chown -R 65532:65532 snapshots
fi

# Prepare database directory
mkdir -p privatedb/coo
mkdir -p privatedb/state
mkdir -p privatedb/hornet-2
mkdir -p privatedb/hornet-3
mkdir -p privatedb/hornet-4
if [[ "$OSTYPE" != "darwin"* ]]; then
  chown -R 65532:65532 privatedb
fi

# Bootstrap coordinator
docker-compose --profile "bootstrap" up
