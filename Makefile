BINARY := airing
IMAGE  := airing-go:dev

.PHONY: build test test-race vet fmt docker-build clean

build:
	go build -trimpath -ldflags="-s -w" -o $(BINARY) ./cmd/airing

test:
	go test ./... -v -count=1

test-race:
	go test ./... -v -race -count=1

vet:
	go vet ./...

fmt:
	gofmt -w .

docker-build:
	docker build -t $(IMAGE) .

clean:
	rm -f $(BINARY) $(BINARY).exe
