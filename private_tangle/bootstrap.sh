#!/bin/bash

./cleanup.sh

mkdir -p privatedb
mkdir -p snapshots
chown -R 65532:65532 privatedb
chown -R 65532:65532 snapshots

docker-compose build
docker-compose run create-snapshots
docker-compose run coo-bootstrap
