#!/bin/bash

BASE_PATH=`pwd`

cd $(dirname $0)

set -e -x

source ../../trap_add.sh
source ../setup_loopback.sh

dir=$(mktemp -d)
trap_add "rm -rf $dir" EXIT
chmod 755 $dir

# Mount the disk and copy in the OEM grub.cfg
cp $BASE_PATH/bin/kurmaos-disk.img $dir/
../lib/disk_util --disk_layout=base mount $dir/kurmaos-disk.img /tmp/rootfs
cp oem-grub.cfg /tmp/rootfs/boot/oem/grub.cfg
../lib/disk_util umount /tmp/rootfs

# Convert the image
qemu-img convert -f raw $dir/kurmaos-disk.img -O vmdk -o adapter_type=lsilogic $dir/kurmaos.vmdk

# Package it up
zip -j $BASE_PATH/bin/kurmaos-vmware.zip kurmaos.vmx $dir/kurmaos.vmdk
