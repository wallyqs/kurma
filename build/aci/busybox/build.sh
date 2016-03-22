#!/bin/bash

BASE_PATH=`pwd`

cd $(dirname $0)

set -e -x

# import TeamCity build number if its available
if [ -n "$TC_BUILD_NUMBER" ]; then
    BUILD_VERSION=$TC_BUILD_NUMBER
fi

# generate a version if none is given
if [ -z ${BUILD_VERSION+x} ]; then
    BUILD_VERSION=$(date +%Y.%m.%d-`git rev-parse HEAD | cut -c1-8`)
fi

dir=$(mktemp -d)
trap "rm -rf $dir" EXIT
chmod 755 $dir

tar -xzf ../../../bin/buildroot.tar.gz -C $dir --exclude=./dev

# clean out some stuff
mkdir $dir/dev
echo -n '' > $dir/etc/hostname
echo -n '' > $dir/etc/hosts
rm $dir/etc/resolv.conf

# generate the aci
go run ../build.go -manifest ./manifest.yaml -root $dir -version $BUILD_VERSION -output $BASE_PATH/$1
