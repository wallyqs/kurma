BASEPATH := $(realpath .)
export BASEPATH

# Set to 1 to use static linking.
STATIC :=
PKG    := ./...
user   := $(shell id -u)
group  := $(shell id -g)
DOCKER := docker run -v ${BASEPATH}:${BASEPATH} -w ${BASEPATH} -e IN_DOCKER=1 -e TMPDIR=/tmp -e GOPATH=${GOPATH} --user=${user}\:${group} apcera/kurma-kernel

ifeq ($(STATIC),1)
export CGO_LDFLAGS := -Wl,-Bstatic -lmount -lblkid -luuid -Wl,-Bdynamic
endif

ifdef $(VERSION)
DOCKER += -e VERSION=${VERSION}
endif

ifeq ($(IN_DOCKER),1)
DOCKER :=
endif

#
# Kurma Binaries
#
.PHONY: kurma-cli kurma-server kurma-init
kurma-cli:
	$(DOCKER) go build -o ${BASEPATH}/bin/$@ cmd/kurma-cli.go
kurma-server:
	$(DOCKER) go build -o ${BASEPATH}/bin/$@ cmd/kurma-server.go
kurma-init:
	$(DOCKER) go build -o ${BASEPATH}/bin/$@ cmd/kurma-init.go

#
# kurmaOS init image
#
bin/kurma-init.tar.gz: kurma-init bin/stager-container.aci bin/console.aci bin/ntp.aci bin/udev.aci bin/busybox.aci bin/cni-netplugin.aci
	$(DOCKER) ./build/kurma-init-tarball.sh

#
# Stager - Container
#
bin/stager-container-main: *
	$(DOCKER) go build -o ${BASEPATH}/$@ stager/container.go
bin/stager-container-init: stager/container/init/init.c
	$(DOCKER) gcc -static -o ${BASEPATH}/$@ stager/container/init/init.c
bin/stager-container-run: *
	$(DOCKER) go build -o ${BASEPATH}/$@ stager/container/cmd/run/main.go
bin/stager-container.aci: bin/stager-container-main bin/stager-container-init bin/stager-container-run
	$(DOCKER) ./build/aci/kurma-stager-container/build.sh $@
.PHONY: stager/container
stager/container: bin/stager-container.aci


#
# ACI Images
#

## kurma-api
.PHONY: kurma-api
kurma-api:
	$(DOCKER) go build -o ${BASEPATH}/bin/$@ cmd/kurma-api.go
	$(DOCKER) ./build/aci/kurma-api/build.sh ./bin/$@

## kurma-upgrader
bin/kurma-upgrader: util/installer/installer.go
	$(DOCKER) go build -o ${BASEPATH}/$@ util/installer/installer.go
bin/kurma-upgrader.aci: bin/kurma-upgrader
	$(DOCKER) ./build/aci/kurma-upgrader/build.sh $@
.PHONY: kurma-upgrader
kurma-upgrader: bin/kurma-upgrader.aci

## busybox
bin/buildroot.tar.gz:
	$(DOCKER) ./build/misc/buildroot/build.sh $@
bin/busybox.aci: bin/buildroot.tar.gz
	$(DOCKER) ./build/aci/busybox/build.sh $@
.PHONY: busybox-aci
busybox-aci: bin/busybox.aci

## cni-netplugin
bin/cni-netplugin-setup: build/aci/cni-netplugin/setup.c
	$(DOCKER) gcc -static -o ${BASEPATH}/$@ build/aci/cni-netplugin/setup.c
bin/cni-netplugin.aci: bin/busybox.aci bin/cni-netplugin-setup
	$(DOCKER) ./build/aci/cni-netplugin/build.sh $@
.PHONY: cni-netplugin-aci
cni-netplugin-aci: bin/cni-netplugin.aci

## ntp
bin/ntp.aci:
	$(DOCKER) ./build/aci/ntp/build.sh $@

## udev
bin/udev.aci:
	$(DOCKER) ./build/aci/udev/build.sh $@

## console
bin/console.aci: kurma-cli
	$(DOCKER) ./build/aci/console/build.sh $@


#
# Testing
#
.PHONY: test
test:
	go test -i $(PKG)
	go test -v $(PKG)
.PHONY: local
local: kurma-cli kurma-server stager/container
