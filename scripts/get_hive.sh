#!/bin/bash

COMMIT=$1
MODULES="app autopeering crypto ds kvstore lo logger objectstorage runtime serializer/v2 web"

if [ -z "$COMMIT" ]
then
    echo "ERROR: no commit hash given!"
    exit 1
fi

for i in $MODULES
do
	go get -u github.com/iotaledger/hive.go/$i@$COMMIT
done

./go_mod_tidy.sh