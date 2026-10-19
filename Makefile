APP=sitemap-checker
VERSION=$(shell cat .version)
LDFLAGS=-ldflags "-X main.version=$(VERSION)"
all: build-linux build-windows build-darwin

build-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o build/$(APP)-linux-amd64-$(VERSION) . 
build-windows:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o build/$(APP)-windows-amd64-$(VERSION).exe .
build-darwin:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o build/$(APP)-darwin-amd64-$(VERSION) .
