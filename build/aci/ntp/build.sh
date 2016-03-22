#!/bin/bash

export BASE_PATH=`pwd`

cd $(dirname $0)

set -e -x

dir=$(mktemp -d)
trap "rm -rf $dir" EXIT
chmod 755 $dir

# copy in the startup script
cp start.sh $dir/
chmod a+x $dir/start.sh

# copy in busybox
cp $(which busybox) $dir/
ln -s busybox $dir/sh
ln -s busybox $dir/ntpd

# setup etc and lib folders
mkdir $dir/etc $dir/lib
ln -s lib $dir/lib64
echo "127.0.0.1 localhost localhost.localdomain" > $ntp/etc/hosts

# copy libnss so it can do dns
cp /etc/nsswitch.conf $dir/etc/
cp /lib/libc.so.6 $dir/lib/
cp /lib/ld-linux-x86-64.so.2 $dir/lib/
cp /lib/libnss_dns-*.so $dir/lib/
cp /lib/libnss_files-*.so $dir/lib/
cp /lib/libresolv-*.so $dir/lib/

# generate ld.so.cache
echo "/lib" > $dir/etc/ld.so.conf
(cd $dir && ldconfig -r . -C etc/ld.so.cache -f etc/ld.so.conf)

# generate the aci
if [ -n "$VERSION" ]; then
    params="-version $VERSION"
fi
go run ../build.go -manifest ./manifest.yaml -root $dir $params -output $BASE_PATH/$1
