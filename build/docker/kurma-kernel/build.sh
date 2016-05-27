#!/bin/bash

set -e -x

source /etc/profile

# update portage
emerge-webrsync
emerge --sync

# Add the linux-firmware package to include when bundling
emerge sys-kernel/linux-firmware

# allow the proper kernel version
echo '=sys-kernel/vanilla-sources-4.4.11 ~amd64' >> /etc/portage/package.accept_keywords
emerge =sys-kernel/vanilla-sources-4.4.11
mv /tmp/kernel.defconfig /usr/src/linux/.config

cd /usr/src/linux

# apply patches
for patchfile in /tmp/*.patch ; do
    patch --batch --forward -p1 < $patchfile
    rm $patchfile
done

# compile it
make olddefconfig
make -j3
make INSTALL_MOD_STRIP="--strip-unneeded" modules_install
cp arch/x86/boot/bzImage /boot/bzImage

# cleanup
rm -rf /usr/portage
rm -rf /var/tmp
