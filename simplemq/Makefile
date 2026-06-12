OPENAPI_MODULE := github.com/sacloud/sacloud-sdk-go
OPENAPI_SUBDIR := api/simplemq/openapi
OPENAPI_FILES := message.yaml queue.yaml

.PHONY: clean test openapi

sakumock-simplemq: go.* *.go cmd/sakumock-simplemq/*.go
	go build -o $@ ./cmd/sakumock-simplemq

clean:
	rm -rf sakumock-simplemq dist/

test:
	go test -v ./...

install:
	go install github.com/sacloud/sakumock/simplemq/cmd/sakumock-simplemq

openapi:
	$(eval MOD_VERSION := $(shell go list -m -f '{{.Version}}' $(OPENAPI_MODULE)))
	$(eval MOD_DIR := $(shell go env GOMODCACHE)/$(OPENAPI_MODULE)@$(MOD_VERSION))
	mkdir -p openapi
	$(foreach f,$(OPENAPI_FILES),rm -f openapi/$(f) && cp $(MOD_DIR)/$(OPENAPI_SUBDIR)/$(f) openapi/$(f);)
	@echo "Copied from $(OPENAPI_MODULE) $(MOD_VERSION) ($(OPENAPI_SUBDIR))"
