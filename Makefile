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

DOCKER_FLAGS := --rm -v ${BASEPATH}:${BASEPATH} -w ${BASEPATH} -e IN_DOCKER=1 -e TMPDIR=/tmp -e GOPATH=${GOPATH}
DOCKER_USER  := --user=${user}\:${group}
DOCKER_IMAGE := apcera/kurma-kernel
DOCKER        = docker run ${DOCKER_FLAGS} ${DOCKER_USER} ${DOCKER_IMAGE}

# Setup command for locally running Kurma on Linux, or running it within Docker
# on OS X.
RUN_CONFIG := build/local-kurmad.yml
RUN_CMD = sudo env RUN_CONFIG=${RUN_CONFIG}
ifeq ($(UNAMES),Darwin)
RUN_CMD = docker run ${DOCKER_FLAGS} -v /tmp -e RUN_CONFIG=${RUN_CONFIG} --privileged -i -t ${DOCKER_IMAGE}
endif

# If STATIC=1 is set in the command line, then configure static compilation for
# libraries other than libc.
ifeq ($(STATIC),1)
export CGO_LDFLAGS := -Wl,-Bstatic -lmount -lblkid -luuid -Wl,-Bdynamic
endif

# If the version flag is set, then ensure it will be passed down to Docker.
ifdef VERSION
DOCKER_FLAGS += -e VERSION=${VERSION}
LDFLAGS += -X github.com/apcera/kurma/pkg/apiclient.version=${VERSION}
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
	@echo 'Downloading kurmaOS services (ntp.aci)'
	@curl -L -o bin/ntp.aci http://ci.kurma.io/repository/download/Artifacts_ACIs_KurmaOSServices/master.tcbuildtag/ntp.aci?guest=1


#
# Common Groupings
#
.PHONY: local
local: kurma-cli kurmad stager/container kurma-api ## Build the pieces typically needed for local development and testing.
.PHONY: run
run: ## Locally run kurmad
	@echo 'Running kurmad'
	@$(RUN_CMD) ./build/run.sh


#
# Kurma Binaries
#
.PHONY: kurma-cli kurmad kurma-init
kurma-cli: LDFLAGS += -linkmode internal
kurma-cli: ## Locally build the kurma-cli binary
	go build -ldflags '${LDFLAGS}' -o ${BASEPATH}/bin/$@ cmd/kurma-cli.go
kurmad:    ## Build the kurmad binary in Docker
	$(DOCKER) go build -ldflags '${LDFLAGS}' -o ${BASEPATH}/bin/$@ cmd/kurmad.go
kurma-init:
	$(DOCKER) go build -ldflags '${LDFLAGS}' -o ${BASEPATH}/bin/$@ cmd/kurma-init.go

#
# kurmaOS init image
#
.PHONY: bin/kurma-init.tar.gz
bin/kurma-init.tar.gz:
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

## console
bin/console.aci: kurma-cli
	$(DOCKER) ./build/aci/console/build.sh $@


#
# VM Images
#
.PHONY: vm-rawdisk vm-virtualbox vm-vmware
vm-rawdisk: ## Build the raw disk image for kurmaOS
vm-rawdisk: DOCKER_USER := --privileged
vm-rawdisk:
	$(DOCKER) ./build/vms/raw_disk.sh
vm-aws: ## Build an AWS image
vm-aws: DOCKER_USER := --privileged
vm-aws:
	$(DOCKER) ./build/vms/aws/oem.sh
	docker run $(DOCKER_FLAGS) apcera/docker-aws-tools ./build/vms/aws/import.sh
vm-openstack: ## Build an OpenStack VM image
vm-openstack: vm-rawdisk
	$(DOCKER) ./build/vms/openstack/build.sh
vm-virtualbox: ## Build a Virtualbox VM image
vm-virtualbox: vm-rawdisk
	$(DOCKER) ./build/vms/virtualbox/build.sh
vm-vmware: ## Build a VMware VM image
vm-vmware: DOCKER_USER := --privileged
vm-vmware: vm-rawdisk
	$(DOCKER) ./build/vms/vmware/build.sh


#
# Testing
#
.PHONY: test
test: ## Locally run the unit tests
	go test -i $(PKG)
	go test -v $(PKG)


#
# Release
#

.PHONY: release release-linux release-darwin
ifeq ($(UNAMES),Linux)
release: release-linux
else ifeq ($(UNAMES),Darwin)
release: release-darwin
endif
release-linux: ## Run release builds for Linux
release-linux: kurma-cli kurmad stager/container kurma-init
release-linux: kurma-api bin/console.aci bin/kurma-init.tar.gz kurma-upgrader
release-linux: vm-rawdisk vm-openstack vm-virtualbox vm-vmware
.PHONY: release-darwin
release-darwin: ## Run release builds for Darwin
release-darwin: kurma-cli

.PHONY: release-to-resources release-to-resources-linux release-to-resources-darwin
ifeq ($(UNAMES),Linux)
release-to-resources: release-to-resources-linux
else ifeq ($(UNAMES),Darwin)
release-to-resources: release-to-resources-darwin
endif
release-to-resources-linux:
	@mkdir -p ./resources
	@cp ./bin/console.aci ./bin/kurma-api.aci ./bin/kurma-cli \
		./bin/kurmaos-openstack.zip ./bin/kurmaos-virtualbox.zip \
		./bin/kurmaos-vmware.zip ./bin/kurmad \
		./bin/kurma-upgrader.aci ./bin/stager-container.aci ./resources/
release-to-resources-darwin:
	@mkdir -p ./resources
	@cp ./bin/kurma-cli ./resources/


#
# Help
#
.PHONY: help
help:
	@grep -E '^[a-zA-Z_/\\-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
