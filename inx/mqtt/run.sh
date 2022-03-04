#!/bin/bash

DIR="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
cd $DIR/../../tools/inx-mqtt
go build -o $DIR/inx-mqtt
cd $DIR
./inx-mqtt