# gitsyncd Makefile. This can build the binary as well as create a release.
all: gitsyncd 

.PHONY: gityncd
gitsyncd:
	GOPATH=`pwd` go install gitsyncd

.PHONY:clean
clean:
	GOPATH=`pwd` go clean
	rm bin/*
