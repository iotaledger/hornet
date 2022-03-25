#!/bin/bash

./cleanup.sh
docker-compose run create-snapshots
docker-compose run coo-bootstrap