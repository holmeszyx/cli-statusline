BINARY := cli-statusline
ifeq ($(OS),Windows_NT)
	BINARY := cli-statusline.exe
endif

.PHONY: build vet test demo clean

build:
	go build -o bin/$(BINARY) ./cmd/cli-statusline

vet:
	go vet ./...

test: vet
	go test -v ./...

demo: build
	./bin/$(BINARY) -demo

clean:
	rm -rf bin
