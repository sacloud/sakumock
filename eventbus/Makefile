OPENAPI_MODULE := github.com/sacloud/sacloud-sdk-go
OPENAPI_SUBDIR := api/eventbus/openapi
OPENAPI_FILES := openapi.json

.PHONY: clean test openapi

sakumock-eventbus: ../go.* *.go cmd/sakumock-eventbus/*.go
	go build -o $@ ./cmd/sakumock-eventbus

clean:
	rm -rf sakumock-eventbus dist/

test:
	go test -v ./...

install:
	go install github.com/sacloud/sakumock/eventbus/cmd/sakumock-eventbus

openapi:
	$(eval MOD_VERSION := $(shell go list -m -f '{{.Version}}' $(OPENAPI_MODULE)))
	$(eval MOD_DIR := $(shell go env GOMODCACHE)/$(OPENAPI_MODULE)@$(MOD_VERSION))
	mkdir -p openapi
	$(foreach f,$(OPENAPI_FILES),rm -f openapi/$(f) && cp $(MOD_DIR)/$(OPENAPI_SUBDIR)/$(f) openapi/$(f);)
	@echo "Copied from $(OPENAPI_MODULE) $(MOD_VERSION) ($(OPENAPI_SUBDIR))"
