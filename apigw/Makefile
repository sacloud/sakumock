# The portal specs are stored as published (sakumock follows the portal).
OPENAPI_URL := https://manual.sakura.ad.jp/api/cloud/portal/openapis/apigw-api.yaml

.PHONY: clean test openapi generate install

sakumock-apigw: ../go.* *.go ../cmd/sakumock-apigw/*.go
	go build -o $@ ../cmd/sakumock-apigw

clean:
	rm -rf sakumock-apigw dist/

test:
	go test -v ./...

install:
	go install github.com/sacloud/sakumock/cmd/sakumock-apigw

openapi:
	mkdir -p openapi
	curl -fsSL -o openapi/openapi.yaml.tmp $(OPENAPI_URL) || { rm -f openapi/openapi.yaml.tmp; exit 1; }
	mv openapi/openapi.yaml.tmp openapi/openapi.yaml
	@echo "Fetched the portal spec(s)"
	$(MAKE) generate

generate:
	go generate .
