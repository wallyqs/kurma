BASEPATH := $(realpath .)
export BASEPATH

# Set to 1 to use static linking.
STATIC :=
PKG    := ./...

# Determine the user ID and group ID to be used within docker. If using Docker
# Toolbox such as on Darwin, it will map to 1000:50.
user   := $(shell id -u)
group  := $(shell id -g)
UNAMES := $(shell uname -s)
ifeq ($(UNAMES),Darwin)
user  := 1000
group := 50
endif
DOCKER := docker run --rm -v ${BASEPATH}:${BASEPATH} -w ${BASEPATH} -e IN_DOCKER=1 -e TMPDIR=/tmp -e GOPATH=${GOPATH} --user=${user}\:${group} apcera/kurma-kernel

# Setup command for locally running Kurma on Linux, or running it within Docker
# on OS X.
RUN_CONFIG := build/local-kurmad.yml
RUN_CMD := sudo env RUN_CONFIG=${RUN_CONFIG}
ifeq ($(UNAMES),Darwin)
RUN_CMD := docker run --rm -v ${BASEPATH}:${BASEPATH} -w ${BASEPATH} -e IN_DOCKER=1 -e TMPDIR=/tmp -v /tmp -e RUN_CONFIG=${RUN_CONFIG} --user=0:0 --privileged -i -t apcera/kurma-kernel
endif

# If STATIC=1 is set in the command line, then configure static compilation for
# libraries other than libc.
ifeq ($(STATIC),1)
export CGO_LDFLAGS := -Wl,-Bstatic -lmount -lblkid -luuid -Wl,-Bdynamic
endif

# If the version flag is set, then ensure it will be passed down to Docker.
ifdef $(VERSION)
DOCKER += -e VERSION=${VERSION}
endif

# If we're already running within in Docker (such as on CI), then blank out the
# Docker command line.
ifeq ($(IN_DOCKER),1)
DOCKER :=
endif

.DEFAULT_GOAL := local

#
# Resources
#
.PHONY: download
download: ## Download common pre-built assets from Kurma's CI
	@echo 'Downloading buildroot.tar.gz'
	@curl -L -o bin/buildroot.tar.gz http://ci.kurma.io/repository/download/Artifacts_ACIs_Buildroot/master.tcbuildtag/buildroot.tar.gz?guest=1
	@echo 'Downloading busybox.aci'
	@curl -L -o bin/busybox.aci http://ci.kurma.io/repository/download/Artifacts_ACIs_Busybox/master.tcbuildtag/busybox.aci?guest=1
	@echo 'Downloading cni-netplugin.aci'
	@curl -L -o bin/cni-netplugin.aci http://ci.kurma.io/repository/download/Artifacts_ACIs_CniNetplugin/master.tcbuildtag/cni-netplugin.aci?guest=1
	@echo 'Downloading kurmaOS services (ntp.aci and udev.aci)'
	@curl -L -o bin/ntp.aci http://ci.kurma.io/repository/download/Artifacts_ACIs_KurmaOSServices/master.tcbuildtag/ntp.aci?guest=1
	@curl -L -o bin/udev.aci http://ci.kurma.io/repository/download/Artifacts_ACIs_KurmaOSServices/master.tcbuildtag/udev.aci?guest=1


#
# Common Groupings
#
.PHONY: local
local: kurma-cli kurma-server stager/container kurma-api ## Build the pieces typically needed for local development and testing.
.PHONY: run
run: ## Locally run kurmad
	@echo 'Running kurmad'
	@$(RUN_CMD) ./build/run.sh


#
# Kurma Binaries
#
.PHONY: kurma-cli kurma-server kurma-init
kurma-cli:    ## Locally build the kurma-cli binary
	go build -o ${BASEPATH}/bin/$@ cmd/kurma-cli.go
kurma-server: ## Build the kurma-server binary in Docker
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
.PHONY: bin/stager-container-main
bin/stager-container-main:
	$(DOCKER) go build -o ${BASEPATH}/$@ stager/container/main.go
bin/stager-container-init: stager/container/init/init.c
	$(DOCKER) gcc -static -o ${BASEPATH}/$@ stager/container/init/init.c
bin/stager-container.aci: bin/stager-container-main bin/stager-container-init
	$(DOCKER) ./build/aci/kurma-stager-container/build.sh $@
.PHONY: stager/container
stager/container: bin/stager-container.aci ## Build the default stager ACI in Docker


#
# ACI Images
#

## kurma-api
.PHONY: kurma-api
kurma-api:
	$(DOCKER) go build -o ${BASEPATH}/bin/$@ cmd/kurma-api.go
	$(DOCKER) ./build/aci/kurma-api/build.sh ./bin/$@.aci

## kurma-upgrader
bin/kurma-upgrader: util/installer/installer.go
	$(DOCKER) go build -o ${BASEPATH}/$@ util/installer/installer.go
bin/kurma-upgrader.aci: bin/kurma-upgrader bin/kurma-init.tar.gz
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
test: ## Locally run the unit tests
	go test -i $(PKG)
	go test -v $(PKG)


#
# Help
#
.PHONY: help
help:
	@grep -E '^[a-zA-Z_-/]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
