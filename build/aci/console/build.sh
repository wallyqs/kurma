#!/bin/bash

export BASE_PATH=`pwd`

cd $(dirname $0)

set -e -x

dir=$(mktemp -d)
trap "rm -rf $dir" EXIT
chmod 755 $dir

mkdir -p $dir/bin $dir/etc $dir/lib $dir/sbin $dir/usr/bin

# configure and build the spawner
cp spawn.json $dir/etc/spawn.conf
cp start.sh $dir/start.sh
chmod a+x $dir/start.sh
go build -a -o $dir/sbin/spawn ../../../vendor/github.com/apcera/util/spawn/spawn.go

# get kurma-cli
cp ../../../bin/kurma-cli $dir/usr/bin/kurma-cli

# create the halt/poweroff/reboot command handler for the container
gcc ../../../util/power/power.c -o $dir/sbin/poweroff
ln -s poweroff $dir/sbin/halt
ln -s poweroff $dir/sbin/reboot

# copy cgpt
cp $(which cgpt) $dir/bin/cgpt

# create a symlink so the console can access kernel modules from the host
ln -s /host/proc/1/root/lib/firmware $dir/lib/firmware
ln -s /host/proc/1/root/lib/modules $dir/lib/modules

# generate the aci
if [ -n "$VERSION" ]; then
    params="-version $VERSION"
fi
go run ../build.go -manifest ./manifest.yaml -root $dir $params -output $BASE_PATH/$1
