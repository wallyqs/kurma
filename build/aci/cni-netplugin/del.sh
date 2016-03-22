#!/bin/sh

set -e

export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

# read stdin
payload=$(mktemp $TMPDIR/cni-command.XXXXXX)
cat > $payload <&0

# locate the plugin
plugin=$(jq -r '.type // ""' < $payload)
if [ "$plugin" == "" ]; then
    echo "No CNI plugin specified"
    exit 1
fi

export CNI_PATH=/usr/bin
export CNI_COMMAND=DEL
export CNI_NETNS=$1
export CNI_CONTAINERID=$2
export CNI_IFNAME=$3

$plugin < $payload
