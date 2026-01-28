.PHONY: build run clean lint setup-hooks

APP_NAME=tg-summary

build:
	go build -o bin/$(APP_NAME) ./cmd/tg-summary

run:
	go run ./cmd/tg-summary

clean:
	rm -rf bin/

lint:
	golangci-lint run

setup-hooks:
	@echo "Setting up git hooks..."
	@chmod +x .git/hooks/pre-commit .git/hooks/pre-push
	@echo "✅ Git hooks installed successfully!"
	@echo "Run 'make lint' to test manually."
