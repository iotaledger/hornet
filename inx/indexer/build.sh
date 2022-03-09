#!/bin/bash

DIR="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
go build -o $DIR/inx-indexer $DIR/../../tools/inx-indexer