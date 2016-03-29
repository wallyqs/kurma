#!/bin/bash

BASE_PATH=`pwd`

cd $(dirname $0)

set -e -x

dir=$(mktemp -d)
trap "rm -rf $dir" EXIT
chmod 755 $dir

qemu-img convert -f raw $BASE_PATH/bin/kurmaos-disk.img -O vmdk -o adapter_type=ide $dir/kurmaos.vmdk

../lib/virtualbox_ovf.sh \
    --vm_name KurmaOS \
    --disk_vmdk $dir/kurmaos.vmdk \
    --memory_size 1024 \
    --output_ovf $dir/kurmaos.ovf

zip -j $BASE_PATH/bin/kurmaos-virtualbox.zip $dir/kurmaos.ovf $dir/kurmaos.vmdk
