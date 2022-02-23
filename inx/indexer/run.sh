#!/bin/bash

DIR="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
cd $DIR/../../tools/inx-indexer
go build -o $DIR/inx-indexer
cd $DIR
./inx-indexer