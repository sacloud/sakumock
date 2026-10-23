# The portal publishes KMS, CloudHSM and SecretManager as one spec;
# internal/splitspec extracts this service's paths and the components they use.
OPENAPI_URL := https://manual.sakura.ad.jp/api/cloud/portal/openapis/security-encryption-api.yaml
OPENAPI_PREFIX := /cloudhsm/

.PHONY: clean test openapi generate

sakumock-cloudhsm: ../go.* *.go cmd/sakumock-cloudhsm/*.go
	go build -o $@ ./cmd/sakumock-cloudhsm

clean:
	rm -rf sakumock-cloudhsm dist/

test:
	go test -v ./...

install:
	go install github.com/sacloud/sakumock/cloudhsm/cmd/sakumock-cloudhsm

openapi:
	mkdir -p openapi
	curl -fsSL -o openapi/.portal.yaml $(OPENAPI_URL)
	go run github.com/sacloud/sakumock/internal/splitspec -prefix $(OPENAPI_PREFIX) -o openapi/openapi.json openapi/.portal.yaml; \
		status=$$?; rm -f openapi/.portal.yaml; exit $$status
	@echo "Extracted $(OPENAPI_PREFIX) from $(OPENAPI_URL)"
	$(MAKE) generate

generate:
	go generate .
