#!/bin/bash

cd /data
fpm -f \
	-p /resources \
	-v $VERSION \
	--deb-systemd /kurmad.service \
	-s dir \
	-t deb \
	-n $PKG_NAME \
	.
