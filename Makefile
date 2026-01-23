.PHONY: build run clean lint

APP_NAME=tg-summary

build:
	go build -o bin/$(APP_NAME) ./cmd/tg-summary

run:
	go run ./cmd/tg-summary

clean:
	rm -rf bin/

lint:
	golangci-lint run
