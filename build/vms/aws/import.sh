#!/bin/bash

BASE_PATH=`pwd`

cd $(dirname $0)

set -e -x

# calculate ldflags for the version number
if [ -z "$VERSION" ]; then
    VERSION="$(git describe --tags | cut -d'-' -f1)+git"
fi

# Import the image into AWS
./do-import.sh -B kurmaos-temp-disk-images \
            -p $BASE_PATH/bin/kurmaos-aws.img \
            -V $VERSION \
            -Z us-west-2a | tee $BASE_PATH/bin/kurmaos-aws-instances.txt

# Remove the no longer needed aws disk
rm $BASE_PATH/bin/kurmaos-aws.img
