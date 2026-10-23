# The portal publishes KMS, CloudHSM and SecretManager as one spec;
# internal/splitspec extracts this service's paths and the components they use.
OPENAPI_URL := https://manual.sakura.ad.jp/api/cloud/portal/openapis/security-encryption-api.yaml
OPENAPI_PREFIX := /secretmanager/

.PHONY: clean test openapi generate

sakumock-secretmanager: ../go.* *.go cmd/sakumock-secretmanager/*.go
	go build -o $@ ./cmd/sakumock-secretmanager

clean:
	rm -rf sakumock-secretmanager dist/

test:
	go test -v ./...

install:
	go install github.com/sacloud/sakumock/secretmanager/cmd/sakumock-secretmanager

openapi:
	mkdir -p openapi
	curl -fsSL -o openapi/.portal.yaml $(OPENAPI_URL)
	go run github.com/sacloud/sakumock/internal/splitspec -prefix $(OPENAPI_PREFIX) -o openapi/openapi.json openapi/.portal.yaml; \
		status=$$?; rm -f openapi/.portal.yaml; exit $$status
	@echo "Extracted $(OPENAPI_PREFIX) from $(OPENAPI_URL)"
	$(MAKE) generate

generate:
	go generate .
