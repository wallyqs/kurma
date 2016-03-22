#!/bin/sh

set -e -o pipefail

device=$1
old_size=$(/bin/blockdev --getsz "${device}")
/bin/cgpt resize "${device}"

# Only resize filesystem if the partition changed
if [[ "${old_size}" -eq $(/bin/blockdev --getsz "${device}") ]]; then
    exit 0
fi

/bin/resize2fs "${device}"
