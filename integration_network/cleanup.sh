#!/bin/bash

if [[ "$OSTYPE" != "darwin"* && "$EUID" -ne 0 ]]; then
  echo "Please run as root or with sudo"
  exit
fi

docker compose down --remove-orphans

rm -Rf privatedb
rm -Rf snapshots