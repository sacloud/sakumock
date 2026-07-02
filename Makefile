.PHONY: clean test install generate

sakumock: go.* cmd/sakumock/*.go
	CGO_ENABLED=0 go build -o $@ ./cmd/sakumock

clean:
	rm -rf sakumock dist/

test:
	go test -v ./...

generate:
	go generate ./...

install:
	go install github.com/sacloud/sakumock/cmd/sakumock
