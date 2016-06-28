#!/bin/bash

cd /data
/usr/local/bin/fpm \
	-f \
	-p /resources \
	-v $VERSION \
	--after-install /startup.sh \
	-s dir \
	-t rpm \
	-n $PKG_NAME \
	.
