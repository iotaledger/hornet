#!/bin/bash

if [ -d "privatedb" ]; then
  echo "Please run 'sudo ./cleanup.sh' first"
  exit 1
fi

mkdir -p privatedb
mkdir -p snapshots
chown -R 65532:65532 privatedb
chown -R 65532:65532 snapshots

docker-compose build
docker-compose run create-snapshots
docker-compose run coo-bootstrap
