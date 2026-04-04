.PHONY: clean test

sakumock-kms: go.* *.go cmd/sakumock-kms/*.go
	go build -o $@ ./cmd/sakumock-kms

clean:
	rm -rf sakumock-kms dist/

test:
	go test -v ./...

install:
	go install github.com/sacloud/sakumock/kms/cmd/sakumock-kms
