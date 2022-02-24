#!/bin/bash

DIR="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
cd $DIR/../../tools/inx-nodejs
npm i
node client.js