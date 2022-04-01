#!/bin/bash

if [ "$EUID" -ne 0 ]
  then echo "Please run as root or with sudo"
  exit
fi

# Cleanup if necessary
if [ -d "privatedb" ]; then
  ./cleanup.sh
fi

# Build latest code
docker-compose build

# Create snapshot
mkdir -p snapshots/coo
chown -R 65532:65532 snapshots
docker-compose run create-snapshots

# Duplicate snapshot for all nodes
cp -R snapshots/coo snapshots/hornet-2
cp -R snapshots/coo snapshots/hornet-3
cp -R snapshots/coo snapshots/hornet-4
chown -R 65532:65532 snapshots

# Prepate database directory
mkdir -p privatedb
chown -R 65532:65532 privatedb

# Bootstrap coordinator
docker-compose run coo-bootstrap
