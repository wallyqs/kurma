#!/bin/bash

BASE_PATH=`pwd`

cd $(dirname $0)

set -e -x

dir=$(mktemp -d)
trap "rm -rf $dir" EXIT
chmod 755 $dir

# setup directories
mkdir -p $dir/lib
ln -s lib $dir/lib64
ln -s /proc/1/root/lib/modules $dir/lib/modules

# main stager binary
cp ../../../bin/stager-container-main $dir/stager

# init container process
mkdir $dir/init
cp ../../../bin/stager-container-init $dir/init/init

# other stager entry points
mkdir -p $dir/opt/stager
cp ../../../bin/stager-container-run $dir/opt/stager/run

# copy some other binaries that may be needed
mkdir $dir/bin
cp $(which busybox) $dir/bin
ln -s busybox $dir/bin/modprobe

# copy needed dynamic libraries
LD_TRACE_LOADED_OBJECTS=1 $dir/stager | grep so | grep -v linux-vdso.so.1 \
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
