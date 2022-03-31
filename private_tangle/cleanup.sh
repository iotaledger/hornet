#!/bin/bash

docker-compose run cleanup
docker-compose down --remove-orphans

rm -Rf privatedb
rm -Rf snapshots