.PHONY: build run test test-race fuzz-smoke lint docs-check verify clean generate

BINARY := vitis
CMD := ./cmd/vitis
MC_VERSION ?= 1.21.4
export MC_VERSION

build:
	go build -o $(BINARY) $(CMD)

run: build
	./$(BINARY) -config configs/vitis.yaml

test:
	go test ./... -count=1

test-race:
	go test -race ./... -count=1

fuzz-smoke:
	go test ./internal/protocol -run=^$$ -fuzz=FuzzBufferBoundedReads -fuzztime=1s
	go test ./internal/nbt -run=^$$ -fuzz=FuzzDecodeRootCompound -fuzztime=1s
	go test ./internal/inventory -run=^$$ -fuzz=FuzzDecodeItemStack -fuzztime=1s -parallel=2
	go test ./internal/inventory -run=^$$ -fuzz=FuzzPersistentItemStack -fuzztime=1s -parallel=2
	go test ./internal/network -run=^$$ -fuzz=FuzzDecompressFrame -fuzztime=1s

lint:
	go vet ./...

docs-check:
	go run ./cmd/vitis-doccheck

verify: test lint test-race fuzz-smoke docs-check

clean:
	rm -f $(BINARY)

generate:
	./scripts/linux/update_version.sh "$$MC_VERSION"
