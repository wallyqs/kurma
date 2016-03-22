#!/bin/bash

BASE_PATH=`pwd`

cd $(dirname $0)

set -e -x

# create the root
dir=$(mktemp -d)
trap "rm -rf $dir" EXIT
chmod 755 $dir

# setup the kurma binary
cp ../bin/kurma-init $dir/kurma
ln -s kurma $dir/init

# copy in the acis
mkdir $dir/acis
cp ../bin/*.aci $dir/acis/

# copy in the kernel modules
rsync -a /lib/modules $dir/lib

# create bin directories
mkdir -p $dir/bin $dir/sbin
cp kurma-init/resizefs.sh $dir/bin/resizefs
chmod a+x $dir/bin/resizefs

# copy busybox and setup necessary links
cp $(which busybox) $dir/bin/busybox
ln -s busybox $dir/bin/blockdev
ln -s busybox $dir/bin/cat
ln -s busybox $dir/bin/grep
ln -s busybox $dir/bin/mktemp
ln -s busybox $dir/bin/modprobe
ln -s busybox $dir/bin/ps
ln -s busybox $dir/bin/rm
ln -s busybox $dir/bin/sh
ln -s busybox $dir/bin/udhcpc
ln -s ../bin/busybox $dir/sbin/ifconfig
ln -s ../bin/busybox $dir/sbin/route

# udhcpc script
mkdir -p $dir/usr/share/udhcpc
cp /usr/share/udhcpc/default.script $dir/usr/share/udhcpc/default.script

# formatting tools
cp /sbin/mke2fs $dir/bin/mke2fs
ln -s mke2fs $dir/bin/mkfs.ext2
ln -s mke2fs $dir/bin/mkfs.ext3
ln -s mke2fs $dir/bin/mkfs.ext4
ln -s mke2fs $dir/bin/mkfs.ext4dev
cp /sbin/resize2fs $dir/bin/resize2fs
cp /usr/bin/cgpt $dir/bin/cgpt

# setup etc
mkdir -p $dir/etc/ssl/certs
cp kurma-init/kurma.json $dir/etc/kurma.json
touch $dir/etc/mtab
touch $dir/etc/resolv.conf
echo "LSB_VERSION=1.4" > $dir/etc/lsb-release
echo "DISTRIB_ID=KurmaOS" >> $dir/etc/lsb-release
echo "DISTRIB_RELEASE=rolling" >> $dir/etc/lsb-release
echo "DISTRIB_DESCRIPTION=KurmaOS" >> $dir/etc/lsb-release
echo "NAME=KurmaOS" > $dir/etc/os-release
echo "VERSION=$version" >> $dir/etc/os-release
echo "ID=kurmaos" >> $dir/etc/os-release
echo "PRETTY_NAME=KurmaOS v$version" >> $dir/etc/os-release

# copy kurma and needed dynamic libraries
ln -s lib $dir/lib64
LD_TRACE_LOADED_OBJECTS=1 $dir/kurma | grep so | grep -v linux-vdso.so.1 \
    | sed -e '/^[^\t]/ d' \
    | sed -e 's/\t//' \
    | sed -e 's/.*=..//' \
    | sed -e 's/ (0.*)//' \
    | xargs -I % cp % $dir/lib/
LD_TRACE_LOADED_OBJECTS=1 $dir/bin/resize2fs | grep so | grep -v linux-vdso.so.1 \
    | sed -e '/^[^\t]/ d' \
    | sed -e 's/\t//' \
    | sed -e 's/.*=..//' \
    | sed -e 's/ (0.*)//' \
    | xargs -I % cp % $dir/lib/

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

# Figure the compresison command
: "${INITRD_COMPRESSION:=gzip}"
compressCommand=""
if [ "$INITRD_COMPRESSION" == "gzip" ]; then
  compressCommand="gzip"
elif [ "$INITRD_COMPRESSION" == "lzma" ]; then
  compressCommand="lzma"
else
  echo "Unrecognized compression setting!"
  exit 1
fi

# package it up
initrdDir=$(mktemp -d /tmp/initrd.XXXXX)
(cd $dir && find . | cpio --quiet -o -H newc | $compressCommand > $initrdDir/initrd)
cp /boot/bzImage $initrdDir/bzImage
cd $initrdDir
tar -czf $BASE_PATH/bin/kurma-init.tar.gz bzImage initrd
