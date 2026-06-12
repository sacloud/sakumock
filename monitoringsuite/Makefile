OPENAPI_MODULE := github.com/sacloud/sacloud-sdk-go
OPENAPI_SUBDIR := api/monitoring-suite/openapi
OPENAPI_FILES := openapi.json

.PHONY: clean test openapi

sakumock-monitoringsuite: ../go.* *.go cmd/sakumock-monitoringsuite/*.go
	go build -o $@ ./cmd/sakumock-monitoringsuite

clean:
	rm -rf sakumock-monitoringsuite dist/

test:
	go test -v ./...

install:
	go install github.com/sacloud/sakumock/monitoringsuite/cmd/sakumock-monitoringsuite

openapi:
	$(eval MOD_VERSION := $(shell go list -m -f '{{.Version}}' $(OPENAPI_MODULE)))
	$(eval MOD_DIR := $(shell go env GOMODCACHE)/$(OPENAPI_MODULE)@$(MOD_VERSION))
	mkdir -p openapi
	$(foreach f,$(OPENAPI_FILES),rm -f openapi/$(f) && cp $(MOD_DIR)/$(OPENAPI_SUBDIR)/$(f) openapi/$(f);)
	@echo "Copied from $(OPENAPI_MODULE) $(MOD_VERSION) ($(OPENAPI_SUBDIR))"
