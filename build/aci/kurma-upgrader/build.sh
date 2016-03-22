#!/bin/bash

BASE_PATH=`pwd`

cd $(dirname $0)

set -e -x

dir=$(mktemp -d)
trap "rm -rf $dir" EXIT
chmod 755 $dir

mkdir $dir/lib
ln -s lib $dir/lib64

# copy binaries
cp ../../../bin/kurma-upgrader $dir/
cp $(which cgpt) $dir/
cp $(which kexec) $dir/

# extract the kurma-init build
tar -xf ../../../bin/kurma-init.tar.gz

# copy needed dynamic libraries
LD_TRACE_LOADED_OBJECTS=1 $dir/kurma-upgrader | grep so | grep -v linux-vdso.so.1 \
    | sed -e '/^[^\t]/ d' \
    | sed -e 's/\t//' \
    | sed -e 's/.*=..//' \
    | sed -e 's/ (0.*)//' \
    | xargs -I % cp % $dir/lib/
LD_TRACE_LOADED_OBJECTS=1 $dir/kexec | grep so | grep -v linux-vdso.so.1 \
    | sed -e '/^[^\t]/ d' \
    | sed -e 's/\t//' \
    | sed -e 's/.*=..//' \
    | sed -e 's/ (0.*)//' \
    | xargs -I % cp % $dir/lib/

# generate the aci
go run ../build.go -manifest ./manifest.yaml -root $dir -output $BASE_PATH/$1
