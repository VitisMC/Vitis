.PHONY: build run test lint clean generate

BINARY := vitis
CMD := ./cmd/vitis

build:
	go build -o $(BINARY) $(CMD)

run: build
	./$(BINARY) -config configs/vitis.yaml

test:
	go test ./internal/... -count=1

lint:
	go vet ./...

clean:
	rm -f $(BINARY)

generate:
	./scripts/update_version.sh 1.21.4
