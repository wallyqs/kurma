#!/bin/bash

BASE_PATH=`pwd`

cd $(dirname $0)

set -e -x

dir=$(mktemp -d)
trap "rm -rf $dir" EXIT
chmod 755 $dir

mkdir $dir/lib
ln -s lib $dir/lib64

cp ../../../bin/kurma-api $dir/

# copy needed dynamic libraries
LD_TRACE_LOADED_OBJECTS=1 $dir/kurma-api | grep so | grep -v linux-vdso.so.1 \
    | sed -e '/^[^\t]/ d' \
    | sed -e 's/\t//' \
    | sed -e 's/.*=..//' \
    | sed -e 's/ (0.*)//' \
    | xargs -I % cp % $dir/lib/

# generate ld.so.cache
mkdir $dir/etc
echo "/lib" > $dir/etc/ld.so.conf
(cd $dir && ldconfig -r . -C etc/ld.so.cache -f etc/ld.so.conf)

# generate the aci
go run ../build.go -manifest ./manifest.yaml -root $dir -output $BASE_PATH/$1
