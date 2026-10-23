# The portal specs are stored as published (sakumock follows the portal).
OPENAPI_URL_MESSAGE := https://manual.sakura.ad.jp/api/cloud/portal/openapis/simplemq-api.yaml
OPENAPI_URL_QUEUE := https://manual.sakura.ad.jp/api/cloud/portal/openapis/simplemq-sacloud-api.yaml

.PHONY: clean test openapi generate

sakumock-simplemq: ../go.* *.go cmd/sakumock-simplemq/*.go
	go build -o $@ ./cmd/sakumock-simplemq

clean:
	rm -rf sakumock-simplemq dist/

test:
	go test -v ./...

install:
	go install github.com/sacloud/sakumock/simplemq/cmd/sakumock-simplemq

openapi:
	mkdir -p openapi
	curl -fsSL -o openapi/message.yaml.tmp $(OPENAPI_URL_MESSAGE) || { rm -f openapi/message.yaml.tmp; exit 1; }
	mv openapi/message.yaml.tmp openapi/message.yaml
	curl -fsSL -o openapi/queue.yaml.tmp $(OPENAPI_URL_QUEUE) || { rm -f openapi/queue.yaml.tmp; exit 1; }
	mv openapi/queue.yaml.tmp openapi/queue.yaml
	@echo "Fetched the portal spec(s)"
	$(MAKE) generate

generate:
	go generate .
