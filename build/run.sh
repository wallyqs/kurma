#!/bin/bash

set -e

if [ -z "$RUN_CONFIG" ]; then
    echo "kurmad configuration file in RUN_CONFIG not set!"
    exit 1
fi

cd $(dirname $0)
BASE_PATH=`pwd`
SOURCE_PATH=$(dirname $BASE_PATH)

dir=$(mktemp -d)
trap "rm -rf $dir" EXIT
chmod 755 $dir

ln -s $SOURCE_PATH $dir/source

ip=$(ifconfig | grep -Eo 'inet (addr:)?([0-9]*\.){3}[0-9]*' | grep -Eo '([0-9]*\.){3}[0-9]*' | grep -v '127.0.0.1')

if [ -n "$IN_DOCKER" ]; then
    echo "=============================================================="
    echo "Kurma remote API will be available at $ip:12312"
    echo
    echo "To connect with kurma-cli, please run:"
    echo "  export KURMA_HOST=$ip"
    echo "=============================================================="
else
    echo "=============================================================="
    echo "Kurma API will be available at $dir/kurma.sock"
    echo
    echo "To connect with kurma-cli, please run:"
    echo "  export KURMA_HOST=$dir/kurma.sock"
    echo "=============================================================="
fi

cd $dir
./source/bin/kurma-server -configFile ./source/$RUN_CONFIG
