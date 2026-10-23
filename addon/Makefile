# The portal specs are stored as published (sakumock follows the portal).
OPENAPI_URL := https://manual.sakura.ad.jp/api/cloud/portal/openapis/addon-api.yaml

.PHONY: clean test openapi generate

sakumock-addon: ../go.* *.go cmd/sakumock-addon/*.go
	go build -o $@ ./cmd/sakumock-addon

clean:
	rm -rf sakumock-addon dist/

test:
	go test -v ./...

install:
	go install github.com/sacloud/sakumock/addon/cmd/sakumock-addon

openapi:
	mkdir -p openapi
	curl -fsSL -o openapi/openapi.yaml.tmp $(OPENAPI_URL) || { rm -f openapi/openapi.yaml.tmp; exit 1; }
	mv openapi/openapi.yaml.tmp openapi/openapi.yaml
	@echo "Fetched the portal spec(s)"
	$(MAKE) generate

generate:
	go generate .
