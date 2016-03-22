#!/bin/bash

export BASE_PATH=`pwd`

cd $(dirname $0)

set -e -x

# compile the cni binaries
cnidir=$(mktemp -d)
trap "rm -rf $cnidir" EXIT
git clone https://github.com/appc/cni.git $cnidir/cni
version=$(cd $cnidir/cni/.git && git describe --tags)
(cd $cnidir/cni && ./build)

# if we're running in TeamCity, then export the version information.
if [ -n "$TC_BUILD_NUMBER" ]; then
    echo "##teamcity[buildNumber '$version']"
fi

dir=$(mktemp -d)
trap "rm -rf $dir" EXIT
chmod 755 $dir

# copy in cni binaries
mkdir -p $dir/usr/bin
cp $cnidir/cni/bin/* $dir/usr/bin/
# except cnitool
rm $dir/usr/bin/cnitool

# copy in the networking script
mkdir -p $dir/opt/network
cp $BASE_PATH/bin/cni-netplugin-setup $dir/opt/network/setup
cp add.sh $dir/opt/network/add
cp del.sh $dir/opt/network/del
chmod a+x $dir/opt/network/*

# generate the aci
go run ../build.go -manifest ./manifest.yaml -root $dir -version $version -output $BASE_PATH/$1
