.PHONY: clean test install

sakumock: go.* cmd/sakumock/*.go
	go build -o $@ ./cmd/sakumock

clean:
	rm -rf sakumock dist/

test:
	go test -v ./...

install:
	go install github.com/sacloud/sakumock/cmd/sakumock
