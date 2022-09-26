#!/bin/bash

TEST_NAMES='common value migration snapshot autopeering'

echo "Build latest HORNET image"
docker build -f ../Dockerfile -t hornet:dev ../.

if ! docker image ls | grep -q wfmock
then
  echo "Pull additional Docker images"
  if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    docker build github.com/iotaledger/chrysalis-tools#:wfmock -t wfmock:latest
  elif [[ "$OSTYPE" == "darwin"* ]]; then
    echo "wfmock:latest needs to be built by hand before running this scripts"
    exit 1
  fi
fi

docker pull iotaledger/inx-coordinator:1.0-rc
docker pull iotaledger/inx-indexer:1.0-rc

echo "Run integration tests"
for name in $TEST_NAMES; do
  echo "Run ${name}"
  TEST_NAME=$name docker compose -f tester/docker-compose.yml up --abort-on-container-exit --exit-code-from tester --build
  docker logs tester &>logs/"$name"_tester.log
  TEST_NAME=$name docker compose -f tester/docker-compose.yml down
done

