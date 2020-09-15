#!/bin/bash
echo "copying assets into shared volume..."
rm -rf /assets/*
cp -rp /tmp/assets/* /assets
chmod 777 /assets/*
echo "assets:"
ls /assets
echo "running tests..."
go test ./tests/"${TEST_NAME}" -v -timeout 30m
exit_code=$?
echo "changing perms on files inside log dir..."
chmod 777 /tmp/logs/*
exit $exit_code