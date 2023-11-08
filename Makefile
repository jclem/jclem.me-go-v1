.PHONY: assets.build assets.clean assets.tag bootstrap check dev

assets.build: node_modules internal/www/public/scripts/app.js internal/www/public/styles/index.css

assets.clean:
	rm -f internal/www/public/scripts/*.js internal/www/public/styles/*.css

assets.tag: assets.clean assets.build
	script/tag-assets

bootstrap: assets.build

check:
	golangci-lint run

dev:
	go build -o ./tmp/main && ./tmp/main

node_modules:
	npm install

www:
	go build -o www .

internal/www/public/scripts/app.js: internal/www/scripts/app.js
	cp internal/www/scripts/app.js internal/www/public/scripts/app.js

internal/www/public/styles/index.css: internal/www/styles/index.css
	npx tailwindcss -i internal/www/styles/index.css -o internal/www/public/styles/index.css
