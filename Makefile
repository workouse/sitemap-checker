APP=sitemap-checker
VERSION=$(shell cat .version)
all: build-linux build-windows build-darwin

build-linux:
	GOOS=linux GOARCH=amd64 go build -o build/$(APP)-linux-amd64-$(VERSION) *.go 
build-windows:
	GOOS=windows GOARCH=amd64 go build -o build/$(APP)-windows-amd64-$(VERSION) *.go
build-darwin:
	GOOS=darwin GOARCH=amd64 go build -o build/$(APP)-darwin-amd64-$(VERSION) *.go
