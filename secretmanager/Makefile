.PHONY: clean test

sakumock-secretmanager: go.* *.go cmd/sakumock-secretmanager/*.go
	go build -o $@ ./cmd/sakumock-secretmanager

clean:
	rm -rf sakumock-secretmanager dist/

test:
	go test -v ./...

install:
	go install github.com/sacloud/sakumock/secretmanager/cmd/sakumock-secretmanager
