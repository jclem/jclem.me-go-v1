.PHONY: check dev

check:
	golangci-lint run

dev:
	air

www:
	go build -o www .

internal/www/public/styles/index.css: internal/www/styles/index.css
	npx tailwindcss -i internal/www/styles/index.css -o internal/www/public/styles/index.css
