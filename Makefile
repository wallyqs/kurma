BASEPATH := $(realpath .)
export BASEPATH

# Set to 1 to use static linking.
STATIC :=
PKG    := ./...
DOCKER := docker run -v ${BASEPATH}:${BASEPATH} -w ${BASEPATH} -e TMPDIR=/tmp -e GOPATH=${GOPATH}

ifeq ($(STATIC),1)
export CGO_LDFLAGS := -Wl,-Bstatic -lmount -lblkid -luuid -Wl,-Bdynamic
endif

#
# Kurma Binaries
#
.PHONY: kurma-cli kurma-server kurma-init
kurma-cli:
	go build -o ${BASEPATH}/bin/$@ cmd/kurma-cli.go
kurma-server:
	go build -o ${BASEPATH}/bin/$@ cmd/kurma-server.go
kurma-init:
	go build -o ${BASEPATH}/bin/$@ cmd/kurma-init.go

#
# kurmaOS init image
#
bin/kurma-init.tar.gz: kurma-init
	$(DOCKER) apcera/kurma-kernel ./build/kurma-init-tarball.sh

#
# Stager - Container
#
bin/stager-container-main: *
	go build -o ${BASEPATH}/$@ stager/container.go
bin/stager-container-init: stager/container/init/init.c
	gcc -static -o ${BASEPATH}/$@ stager/container/init/init.c
bin/stager-container-run: *
	go build -o ${BASEPATH}/$@ stager/container/cmd/run/main.go
bin/stager-container.aci: bin/stager-container-main bin/stager-container-init bin/stager-container-run
	./build/aci/kurma-stager-container/build.sh $@
.PHONY: stager/container
stager/container: bin/stager-container.aci


#
# ACI Images
#

## kurma-api
.PHONY: kurma-api
kurma-api:
	go build -o ${BASEPATH}/bin/$@ cmd/kurma-api.go
	./build/aci/kurma-api/build.sh ./bin/$@

## kurma-upgrader
bin/kurma-upgrader: util/installer/installer.go
	go build -o ${BASEPATH}/$@ util/installer/installer.go
bin/kurma-upgrader.aci: bin/kurma-upgrader
	./build/aci/kurma-upgrader/build.sh $@
.PHONY: kurma-upgrader
kurma-upgrader: bin/kurma-upgrader.aci

## busybox
bin/buildroot.tar.gz:
	./build/misc/buildroot/build.sh $@
bin/busybox.aci: bin/buildroot.tar.gz
	./build/aci/busybox/build.sh $@
.PHONY: busybox-aci
busybox-aci: bin/busybox.aci

## cni-netplugin
bin/cni-netplugin-setup: build/aci/cni-netplugin/setup.c
	gcc -static -o ${BASEPATH}/$@ build/aci/cni-netplugin/setup.c
bin/cni-netplugin.aci: bin/busybox.aci bin/cni-netplugin-setup
	./build/aci/cni-netplugin/build.sh $@
.PHONY: cni-netplugin-aci
cni-netplugin-aci: bin/cni-netplugin.aci

## ntp
bin/ntp.aci:
	$(DOCKER) -e VERSION=${VERSION} apcera/kurma-stage4 ./build/aci/ntp/build.sh $@

## udev
bin/udev.aci:
	$(DOCKER) -e VERSION=${VERSION} apcera/kurma-stage4 ./build/aci/udev/build.sh $@


#
# Testing
#
.PHONY: test
test:
	go test -i $(PKG)
	go test -v $(PKG)
.PHONY: local
local: kurma-cli kurma-server stager/container
