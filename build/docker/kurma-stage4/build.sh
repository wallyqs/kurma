#!/bin/bash

set -e -x

source /etc/profile

echo 'GRUB_PLATFORMS="efi-64 pc xen"' >> /etc/portage/make.conf
# Disable sandboxing. This was causing issues building python modules pulled
# in by xen being in GRUB_PLATFORMS. The modules were throwing access
# violations for the sandbox only when being installed within a container, not
# when in a normal chroot.
echo 'FEATURES="-sandbox -usersandbox"' >> /etc/portage/make.conf

# Enable building static libraries when installing packages.
echo 'USE="$USE static-libs"' >> /etc/portage/make.conf

##
## LOCAL USE FLAGS
##

# eudev use kmod
echo 'sys-fs/eudev kmod' >> /etc/portage/package.use/eudev


# update portage
emerge-webrsync
emerge --sync

# install layman
emerge app-portage/layman

# Add in the Apcera overlay. This contains specific ebuilds which we'll want
# to reference.
layman -o https://raw.githubusercontent.com/apcera/kurmaos-overlay/master/overlay.xml -f -a kurmaos-overlay
echo 'source /var/lib/layman/make.conf' >> /etc/portage/make.conf
echo 'kurmaos-base' >> /etc/portage/categories
echo "=app-emulation/open-vm-tools-9.10.0" >> /etc/portage/package.unmask
echo "=kurmaos-base/vboot_reference-2.1.0" >> /etc/portage/package.unmask
echo "=dev-libs/libdnet-1.12" >> /etc/portage/package.unmask
echo "=dev-libs/libmspack-0.4_alpha" >> /etc/portage/package.unmask
echo "=sys-boot/grub-2.02_beta2_p20150727-r1" >> /etc/portage/package.unmask
echo "=sys-boot/syslinux-4.07-r1" >> /etc/portage/package.unmask

emerge \
    =kurmaos-base/vboot_reference-2.1.0 \
    =app-emulation/open-vm-tools-9.10.0 \
    =dev-lang/go-1.6.2 \
    =sys-boot/grub-2.02_beta2_p20151217-r1 \
    =sys-boot/syslinux-4.07-r1

emerge \
    app-arch/cpio \
    app-arch/zip \
    sys-apps/busybox \
    sys-apps/kexec-tools \
    sys-fs/e2fsprogs \
    sys-fs/dosfstools \
    app-emulation/qemu \
    app-misc/jq \
    dev-vcs/mercurial \
    sys-devel/bc \
    sys-apps/hwids \
    sys-fs/eudev

# Rebuild util-linux so it has static libraries. First need to remove
# mount/umount to avoid Gentoo errors about suspicious suid hardlinks.
rm -f /bin/mount /bin/umount
emerge sys-apps/util-linux

# install acbuild, for creating aci images
curl -L https://github.com/appc/acbuild/releases/download/v0.2.2/acbuild.tar.gz | tar xzv -C /usr/bin

# cleanup
rm -rf /usr/portage
rm -rf /var/tmp
