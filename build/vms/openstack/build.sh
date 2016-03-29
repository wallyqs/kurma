#!/bin/bash

BASE_PATH=`pwd`

cd $(dirname $0)

set -e -x

dir=$(mktemp -d)
trap "rm -rf $dir" EXIT
chmod 755 $dir

qemu-img convert -f raw $BASE_PATH/bin/kurmaos-disk.img -O qcow2 -o compat=0.10 $dir/kurmaos.img

zip -j $BASE_PATH/bin/kurmaos-openstack.zip $dir/kurmaos.img
