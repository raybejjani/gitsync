# gitsyncd Makefile. This can build the binary as well as create a release.
all: gitsyncd 

.PHONY: gityncd gitsyncd_noweb
gitsyncd: prep_web_files gitsyncd_noweb 

	@go get -tags='makebuild' github.com/raybejjani/gitsync/gitsyncd
gitsyncd_noweb: version 
	@go install -tags='makebuild' github.com/raybejjani/gitsync/gitsyncd

.PHONY: version
version:
	@# pass

.PHONY: prep_web_files
prep_web_files:
	./build-util/make_code_fs.py \
		--index ./web/templates/index.html \
		-i ${GOPATH}/src/github.com/raybejjani/gitsync/gitsyncd/webcontent/content_base.go \
		-o ${GOPATH}/src/github.com/raybejjani/gitsync/gitsyncd/webcontent/content.go \
		web 

.PHONY:clean
clean:
	@go clean
	@rm bin/*
