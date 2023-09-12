-include Makefile.options
-include version
#####################################################################################
## print usage information
help:
	@echo 'Usage:'
	@cat ${MAKEFILE_LIST} | grep -e "^## " -A 1 | grep -v '\-\-' | sed 's/^##//' | cut -f1 -d":" | \
		awk '{info=$$0; getline; print "  " $$0 ": " info;}' | column -t -s ':' | sort 
.PHONY: help
#####################################################################################
## call units tests
test/unit: 
	go test -race -count 1 ./...
.PHONY: test/unit
#####################################################################################
## code vet and lint
test/lint: 
	go vet ./...
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	golangci-lint run -v ./...
.PHONY: test/lint
#####################################################################################
#####################################################################################
## cleans go mod
clean:
	go mod tidy -compat=1.21
	go clean
