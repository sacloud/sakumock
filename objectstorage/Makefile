OPENAPI_MODULE := github.com/sacloud/sacloud-sdk-go
OPENAPI_SUBDIR := api/object-storage/openapi
OPENAPI_FILES := openapi.json

.PHONY: clean test openapi

sakumock-objectstorage: ../go.* *.go cmd/sakumock-objectstorage/*.go
	go build -o $@ ./cmd/sakumock-objectstorage

clean:
	rm -rf sakumock-objectstorage dist/

test:
	go test -v ./...

install:
	go install github.com/sacloud/sakumock/objectstorage/cmd/sakumock-objectstorage

openapi:
	$(eval MOD_VERSION := $(shell go list -m -f '{{.Version}}' $(OPENAPI_MODULE)))
	$(eval MOD_DIR := $(shell go env GOMODCACHE)/$(OPENAPI_MODULE)@$(MOD_VERSION))
	mkdir -p openapi
	$(foreach f,$(OPENAPI_FILES),rm -f openapi/$(f) && cp $(MOD_DIR)/$(OPENAPI_SUBDIR)/$(f) openapi/$(f);)
	@echo "Copied from $(OPENAPI_MODULE) $(MOD_VERSION) ($(OPENAPI_SUBDIR))"
