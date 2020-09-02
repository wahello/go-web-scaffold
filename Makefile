VERSION := $(shell git describe --tags --dirty)
BUILD := $(shell date +%FT%T%z)

.PHONY: config clean dir all

all: clean telescope

dir:
	mkdir -p bin && \
	mkdir -p bin/frontend

clean:
	rm -rf bin

telescope: telescope.bin

%.bin: dir
	cd cmd/$* && \
	go build -tags=jsoniter -trimpath -ldflags "-s -w -X telescope/version.Version=$(VERSION) -X telescope/version.BuildDate=$(BUILD)" && \
	cp $* $(PWD)/bin/$*