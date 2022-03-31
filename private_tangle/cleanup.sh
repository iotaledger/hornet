#!/bin/bash

mkdir -p privatedb
mkdir -p snapshots
chown -R 65532:65532 /privatedb
chown -R 65532:65532 /snapshots

docker-compose run cleanup
docker-compose down --remove-orphans