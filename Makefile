.PHONY: check dev

check:
	golangci-lint run

dev:
	air
