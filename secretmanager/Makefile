OPENAPI_MODULE := github.com/sacloud/secretmanager-api-go
OPENAPI_FILES := openapi-fixed.json

.PHONY: clean test openapi

sakumock-secretmanager: go.* *.go cmd/sakumock-secretmanager/*.go
	go build -o $@ ./cmd/sakumock-secretmanager

clean:
	rm -rf sakumock-secretmanager dist/

test:
	go test -v ./...

install:
	go install github.com/sacloud/sakumock/secretmanager/cmd/sakumock-secretmanager

openapi:
	$(eval MOD_VERSION := $(shell go list -m -f '{{.Version}}' $(OPENAPI_MODULE)))
	$(eval MOD_DIR := $(shell go env GOMODCACHE)/$(OPENAPI_MODULE)@$(MOD_VERSION))
	mkdir -p openapi
	$(foreach f,$(OPENAPI_FILES),rm -f openapi/$(f) && cp $(MOD_DIR)/openapi/$(f) openapi/$(f);)
	@echo "Copied from $(OPENAPI_MODULE) $(MOD_VERSION)"
