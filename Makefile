.PHONY: build run dev clean test test-v

build:
	go build -o ai-lab .

run: build
	./ai-lab

dev:
	go run .

clean:
	rm -f ai-lab

test:
	go test ./...

test-v:
	go test -v ./...
