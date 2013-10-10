# gitsyncd Makefile. This can build the binary as well as create a release.
all: gitsyncd 

.PHONY: gityncd gitsyncd_noweb
gitsyncd: prep_web_files gitsyncd_noweb 

	@GOPATH=`pwd` go get -tags='makebuild' gitsyncd
gitsyncd_noweb: version 
	@GOPATH=`pwd` go install -tags='makebuild' gitsyncd

.PHONY: version
version:
	@# pass

.PHONY: prep_web_files
prep_web_files:
	./util/make_code_fs.py \
		--index /templates/index.html \
		-i src/gitsyncd/webcontent/content_base.go \
		-o src/gitsyncd/webcontent/content.go \
		web 2> /dev/null

.PHONY:clean
clean:
	@GOPATH=`pwd` go clean
	@rm bin/*
