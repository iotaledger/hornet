#!/bin/bash
read -p "WARNING! This will remove other unused containers as well.
Are you sure you want to continue? [y/N] " -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]
then
    # handle exits from shell or function but don't exit interactive shell
    [[ "$0" = "$BASH_SOURCE" ]] && exit 1 || return 1
fi

docker ps -a | grep replica_ | cut -d ' ' -f 1 | xargs docker stop
docker stop wfmock
docker ps -a | grep Exit | cut -d ' ' -f 1 | xargs docker rm
docker rmi -f $(docker images -qf "dangling=true")
docker network prune
