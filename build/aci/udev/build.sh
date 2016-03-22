#!/bin/bash

export BASE_PATH=`pwd`

cd $(dirname $0)

set -e -x

dir=$(mktemp -d)
trap "rm -rf $dir" EXIT
chmod 755 $dir

# copy in the startup script
cp start.sh $dir/
chmod a+x start.sh

# copy in busybox and udev
mkdir $dir/bin
cp $(which busybox) $dir/bin/
ln -s busybox $dir/bin/sh
cp $(which udevd) $dir/bin/
cp $(which udevadm) $dir/bin/

# setup etc and lib folders
mkdir $dir/etc $dir/lib $dir/run
ln -s lib $dir/lib64
echo "127.0.0.1 localhost localhost.localdomain" > $dir/etc/hosts

# copy the udev rules
cp -r /lib/udev $dir/lib/

# setup some of our own
mkdir -p $dir/etc/udev/rules.d
touch $dir/etc/udev/rules.d/80-net-name-slot.rules
ln -s /lib/udev/hwdb.d $dir/etc/udev/hwdb.d

# copy dynamic libraries
LD_TRACE_LOADED_OBJECTS=1 $dir/bin/udevd | grep so | grep -v linux-vdso.so.1 \
    | sed -e '/^[^\t]/ d' \
    | sed -e 's/\t//' \
    | sed -e 's/.*=..//' \
    | sed -e 's/ (0.*)//' \
    | xargs -I % cp % $dir/lib/
LD_TRACE_LOADED_OBJECTS=1 $dir/bin/udevadm | grep so | grep -v linux-vdso.so.1 \
    | sed -e '/^[^\t]/ d' \
    | sed -e 's/\t//' \
    | sed -e 's/.*=..//' \
    | sed -e 's/ (0.*)//' \
    | xargs -I % cp % $dir/lib/

# generate ld.so.cache
echo "/lib" > $dir/etc/ld.so.conf
(cd $dir && ldconfig -r . -C etc/ld.so.cache -f etc/ld.so.conf)

# create a symlink so the udev can access kernel modules from the host
ln -s /host/proc/1/root/lib/modules $dir/lib/modules
ln -s /host/proc/1/root/lib/firmware $dir/lib/firmware

# update the hardware db
udevadm hwdb --update --root=$dir

# generate the aci
if [ -n "$VERSION" ]; then
    params="-version $VERSION"
fi
go run ../build.go -manifest ./manifest.yaml -root $dir $params -output $BASE_PATH/$1
