################################################################################
# This Makefile exists for developers and is not integral to deployment.
################################################################################

all:
	@echo Valid targets are: '"vet"', '"fmt"', '"test"' and '"testrace"'
	@echo There is also a Makefile in the _examples sub-directory
	@echo

.PHONY: vet
vet:
	go vet ./...

.PHONY: fmt
fmt:
	gofmt -s -w .

.PHONY: test tests
test tests:
	go vet ./...
	go test ./...

.PHONY: testrace
testrace:
	go test -race ./...
