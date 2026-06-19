.PHONY: build test vet fmt fmt-check tidy run install clean check hooks

# build the marty binary into the repo root (gitignored); run it with ./marty
build:
	go build -o marty ./cmd/marty

# run tests
test:
	go test ./...

vet:
	go vet ./...

# format in place
fmt:
	gofmt -w .

# fail if anything is unformatted (used by CI + the pre-commit hook)
fmt-check:
	@unformatted=$$(gofmt -l .); if [ -n "$$unformatted" ]; then echo "needs gofmt:"; echo "$$unformatted"; exit 1; fi

tidy:
	go mod tidy

# run the CLI: make run ARGS='book "the will of the many"'
run:
	go run ./cmd/marty $(ARGS)

# install the marty binary onto your PATH
install:
	go install ./cmd/marty

clean:
	rm -f marty

# build + vet + gofmt + test — the gate CI and the pre-commit hook run (leaves no binary)
check: fmt-check vet test
	go build ./...

# enable the repo's git hooks (one-time, per clone)
hooks:
	git config core.hooksPath .githooks
	@echo "git hooks enabled (.githooks)"
