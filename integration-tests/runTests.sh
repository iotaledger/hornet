#!/bin/bash

TEST_NAMES='autopeering common value benchmark snapshot migration'

echo "Build HORNET image"
docker build -f ../docker/Dockerfile -t hornet:dev ../.

echo "Pull additional Docker images"
docker pull gaiaadm/pumba:0.7.4
docker pull gaiadocker/iproute2:latest
docker build github.com/iotaledger/chrysalis-tools#:wfmock -t wfmock:latest

echo "Run integration tests"

for name in $TEST_NAMES; do
  TEST_NAME=$name docker-compose -f tester/docker-compose.yml up --abort-on-container-exit --exit-code-from tester --build
  docker logs tester &>logs/"$name"_tester.log
done

echo "Clean up"
docker-compose -f tester/docker-compose.yml down
docker rm -f $(docker ps -a -q -f ancestor=gaiadocker/iproute2)
