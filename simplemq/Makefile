.PHONY: clean test

sakumock-simplemq: go.* *.go cmd/sakumock-simplemq/*.go
	go build -o $@ ./cmd/sakumock-simplemq

clean:
	rm -rf sakumock-simplemq dist/

test:
	go test -v ./...

install:
	go install github.com/sacloud/sakumock/simplemq/cmd/sakumock-simplemq
