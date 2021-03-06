ifeq ($(origin VERSION), undefined)
  VERSION=$(git rev-parse --short HEAD)
endif

GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)
REPOPATH = kubernetes-ldap

build: vendor
	go build -o bin/kubernetes-ldap -ldflags "-X $(REPOPATH).Version=$(VERSION)" ./cmd/kubernetes-ldap.go

run:
	./bin/kubernetes-ldap

dep:
	curl -o dep -L https://github.com/golang/dep/releases/download/v0.3.2/dep-${GOOS}-${GOARCH}
	chmod +x dep

vendor: dep
	./dep ensure
	./dep status
