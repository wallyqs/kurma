#!/bin/bash

BASE_PATH=`pwd`

cd $(dirname $0)

set -e -x

# Mount the disk and copy in the OEM grub.cfg
cp $BASE_PATH/bin/kurmaos-disk.img $BASE_PATH/bin/kurmaos-aws.img
../lib/disk_util --disk_layout=base mount $BASE_PATH/bin/kurmaos-aws.img /tmp/rootfs
cp oem-grub.cfg /tmp/rootfs/boot/oem/grub.cfg
../lib/disk_util umount /tmp/rootfs
