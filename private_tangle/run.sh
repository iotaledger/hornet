#!/bin/bash

if [ ! -d "privatedb" ]; then
  echo "Please run './bootstrap.sh' first"
  exit
fi

if [ $# -eq 0 ]; then
    docker-compose --profile "2-nodes" up
elif [[ $1 = "3" || $1 = "4" ]]; then
    docker-compose --profile "$1-nodes" up
else
  echo "Usage: ./run.sh [3|4]"
fi