OPENAPI_MODULE := github.com/sacloud/simple-notification-api-go
OPENAPI_FILES := openapi.yaml

.PHONY: clean test openapi

sakumock-simplenotification: go.* *.go cmd/sakumock-simplenotification/*.go
	go build -o $@ ./cmd/sakumock-simplenotification

clean:
	rm -rf sakumock-simplenotification dist/

test:
	go test -v ./...

install:
	go install github.com/sacloud/sakumock/simplenotification/cmd/sakumock-simplenotification

openapi:
	$(eval MOD_VERSION := $(shell go list -m -f '{{.Version}}' $(OPENAPI_MODULE)))
	$(eval MOD_DIR := $(shell go env GOMODCACHE)/$(OPENAPI_MODULE)@$(MOD_VERSION))
	mkdir -p openapi
	$(foreach f,$(OPENAPI_FILES),rm -f openapi/$(f) && cp $(MOD_DIR)/openapi/$(f) openapi/$(f);)
	@echo "Copied from $(OPENAPI_MODULE) $(MOD_VERSION)"
