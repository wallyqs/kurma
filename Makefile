BASEPATH := $(realpath .)
export BASEPATH

# Set to 1 to use static linking.
STATIC :=

ifeq ($(STATIC),1)
export CGO_LDFLAGS := -Wl,-Bstatic -lmount -lblkid -luuid -Wl,-Bdynamic
endif

#
# Kurma Binaries
#
bin/kurma-cli: *
	go build -o ${BASEPATH}/$@ cmd/kurma-cli.go
bin/kurma-server: *
	go build -o ${BASEPATH}/$@ cmd/kurma-server.go
bin/kurma-init: *
	go build -o ${BASEPATH}/$@ cmd/kurma-init.go
.PHONY: kurma
kurma: bin/kurma-cli bin/kurma-server bin/kurma-init
.PHONY: local
local: bin/kurma-cli bin/kurma-server

#
# Stager - Container
#
bin/stager-container-main: *
	go build -o ${BASEPATH}/$@ stager/container.go
bin/stager-container-init: stager/container/init/init.c
	gcc -static -o ${BASEPATH}/$@ stager/container/init/init.c
bin/stager-container-run: *
	go build -o ${BASEPATH}/$@ stager/container/cmd/run/main.go
.PHONY: stager/container
stager/container: bin/stager-container-main bin/stager-container-init \
	bin/stager-container-run
