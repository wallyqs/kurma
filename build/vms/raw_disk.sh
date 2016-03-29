#!/bin/bash

BASE_PATH=`pwd`

cd $(dirname $0)

set -e -x

# Create and mount the disk
./lib/disk_util --disk_layout=base format $BASE_PATH/bin/kurmaos-disk.img
./lib/disk_util --disk_layout=base mount $BASE_PATH/bin/kurmaos-disk.img /tmp/rootfs
mkdir -p /tmp/rootfs/boot/kurmaos

tar -xf $BASE_PATH/bin/kurma-init.tar.gz --owner root --group root --no-same-owner -C /tmp/rootfs/boot/kurmaos
mv /tmp/rootfs/boot/kurmaos/bzImage /tmp/rootfs/boot/kurmaos/vmlinuz-a
mv /tmp/rootfs/boot/kurmaos/initrd /tmp/rootfs/boot/kurmaos/initrd-a

./lib/disk_util umount /tmp/rootfs

for target in i386-pc x86_64-efi x86_64-xen; do
    ./lib/grub_install.sh --target=$target --disk_image=$BASE_PATH/bin/kurmaos-disk.img
done
