.PHONY: assets check dev

assets: internal/www/public/scripts/app.js internal/www/public/styles/index.css
	script/tag-assets

check:
	golangci-lint run

dev:
	air

www:
	go build -o www .

internal/www/public/scripts/app.js: internal/www/scripts/app.js
	cp internal/www/scripts/app.js internal/www/public/scripts/app.js

internal/www/public/styles/index.css: internal/www/styles/index.css
	npx tailwindcss -i internal/www/styles/index.css -o internal/www/public/styles/index.css
