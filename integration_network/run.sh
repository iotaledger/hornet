#!/bin/bash

if [ ! -d "privatedb" ]; then
  echo "Please run './bootstrap.sh' first"
  exit
fi

# first argument must be '3|4' and extra argument must not exist or must start with '-'
if [[ ($1 = "3" || $1 = "4") && ($2 = "" || $2 = -*) ]]; then
    PROFILE=$1
    # shift arguments to remove profile arg
    shift;
    docker compose --profile "$PROFILE-nodes" up $@

# argument must not exist or must start with '-'
elif [[ $1 = "" || $1 = -* ]]; then
    docker compose --profile "2-nodes" up $@ 
else
  echo "Usage: ./run.sh [3|4] [docker compose up options]"
fi