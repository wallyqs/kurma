#!/bin/sh

PACKAGE_DIR=$(mktemp -d)
trap "rm -rf $PACKAGE_DIR" EXIT

mkdir -p $PACKAGE_DIR/usr/bin \
	$PACKAGE_DIR/usr/share/kurmad \
	$PACKAGE_DIR/etc/kurmad \
	$PACKAGE_DIR/var/cache/kurmad/images \
	$PACKAGE_DIR/var/cache/kurmad/volumes \
	$PACKAGE_DIR/var/cache/kurmad/pods \
	$PACKAGE_DIR/var/run \
	$PACKAGE_DIR/lib/systemd/system
cp ./bin/kurma-cli ./bin/kurmad $PACKAGE_DIR/usr/bin
cp ./build/release/base-config.yml $PACKAGE_DIR/etc/kurmad/config.yml
cp ./bin/kurma-api.aci \
	./bin/console.aci \
	./bin/kurma-upgrader.aci \
	./bin/stager-container.aci \
	./bin/busybox.aci \
	./bin/cni-netplugin.aci \
	$PACKAGE_DIR/usr/share/kurmad
cp ./build/release/kurmad.service $PACKAGE_DIR/lib/systemd/system
# note the use of $(pwd) in the bind mount of the entrypoint.sh and startup.sh
# files as volumes. Docker does not support relative paths for files mounted
# as volumes.
docker run -v $PACKAGE_DIR:/data \
	   -v $(pwd)/build/docker/rpm/entrypoint.sh:/entrypoint.sh \
	   -v $(pwd)/build/docker/rpm/startup.sh:/startup.sh \
	   -v $(pwd)/resources:/resources \
	   -e VERSION=$VERSION \
	   -e PKG_NAME="kurmad-systemd" \
	   kurma/centos-fpm:latest
