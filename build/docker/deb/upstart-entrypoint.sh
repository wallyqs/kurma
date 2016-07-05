#!/bin/bash

cd /data
fpm -f \
	-p /resources \
	-v $VERSION \
	--deb-upstart /kurmad.conf \
	--after-install /upstart-postinst.sh \
	-s dir \
	-t deb \
	-n $PKG_NAME \
	.
