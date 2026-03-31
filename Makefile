.PHONY: build run test vet clean

build:
	CGO_ENABLED=0 go build -o brand ./cmd/brand/

run: build
	BRAND_ADMIN_KEY=dev-admin DATA_DIR=./data ./brand

test:
	go test ./... -count=1 -timeout 60s

vet:
	go vet ./...

clean:
	rm -f brand
