#!/bin/bash

PACKAGE_DIR=$(mktemp -d)
trap "rm -rf $PACKAGE_DIR" EXIT

mkdir -p $PACKAGE_DIR/usr/bin \
	$PACKAGE_DIR/usr/share/kurmad \
	$PACKAGE_DIR/etc/kurmad \
	$PACKAGE_DIR/var/cache/kurmad/images \
	$PACKAGE_DIR/var/cache/kurmad/volumes \
	$PACKAGE_DIR/var/cache/kurmad/pods \
	$PACKAGE_DIR/var/run
cp ./bin/kurma-cli ./bin/kurmad $PACKAGE_DIR/usr/bin
cp ./build/release/base-config.yml $PACKAGE_DIR/etc/kurmad/config.yml
cp ./bin/kurma-api.aci \
	./bin/console.aci \
	./bin/kurma-upgrader.aci \
	./bin/stager-container.aci \
	./bin/busybox.aci \
	./bin/cni-netplugin.aci \
	$PACKAGE_DIR/usr/share/kurmad

docker run -v $PACKAGE_DIR:/data \
	   -v $(pwd)/build/docker/deb/systemd-entrypoint.sh:/entrypoint.sh \
	   -v $(pwd)/build/release/kurmad.service:/kurmad.service \
	   -v $(pwd)/resources:/resources \
	   -e VERSION=$VERSION \
	   -e PKG_NAME="kurmad-systemd" \
	   kurma/debian-fpm

docker run -v $PACKAGE_DIR:/data \
	   -v $(pwd)/build/docker/deb/upstart-entrypoint.sh:/entrypoint.sh \
	   -v $(pwd)/build/release/kurmad.conf:/kurmad.conf \
	   -v $(pwd)/build/docker/deb/upstart-postinst.sh:/upstart-postinst.sh \
	   -v $(pwd)/resources:/resources \
	   -e VERSION=$VERSION \
	   -e PKG_NAME="kurmad-upstart" \
	   kurma/debian-fpm:latest
