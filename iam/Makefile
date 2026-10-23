# The portal specs are stored as published (sakumock follows the portal).
OPENAPI_URL := https://manual.sakura.ad.jp/api/cloud/portal/openapis/iam-api.yaml

.PHONY: clean test openapi generate

sakumock-iam: ../go.* *.go cmd/sakumock-iam/*.go
	go build -o $@ ./cmd/sakumock-iam

clean:
	rm -rf sakumock-iam dist/

test:
	go test -v ./...

install:
	go install github.com/sacloud/sakumock/iam/cmd/sakumock-iam

openapi:
	mkdir -p openapi
	curl -fsSL -o openapi/openapi.yaml.tmp $(OPENAPI_URL) || { rm -f openapi/openapi.yaml.tmp; exit 1; }
	mv openapi/openapi.yaml.tmp openapi/openapi.yaml
	@echo "Fetched the portal spec(s)"
	$(MAKE) generate

generate:
	go generate .
