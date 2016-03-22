#!/bin/bash

BASE_PATH=`pwd`
cd $(dirname $0)
BUILDROOT_PATH=`pwd`

set -e -x

dir=$(mktemp -d)
trap "rm -rf $dir" EXIT
chmod 755 $dir
cd $dir

# download buildroot
wget https://buildroot.org/downloads/buildroot-2016.02.tar.gz
tar -xf buildroot-2016.02.tar.gz

# setup config files
cp $BUILDROOT_PATH/buildroot.config buildroot-2016.02/.config
cp $BUILDROOT_PATH/busybox.config buildroot-2016.02/busybox.config

# build
cd buildroot-2016.02
make oldconfig
make --quiet

# compress
cp output/images/rootfs.tar.gz $BASE_PATH/$1
