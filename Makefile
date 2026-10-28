.PHONY: build test vet run safe package
build:
	bash scripts/build.sh
test:
	CGO_ENABLED=0 go test ./...
vet:
	CGO_ENABLED=0 go vet ./...
run: build
	build/$$(go env GOOS)-$$(go env GOARCH)/flclash
safe: build
	build/$$(go env GOOS)-$$(go env GOARCH)/flclash --safe-mode
package: build
	bash scripts/package.sh
