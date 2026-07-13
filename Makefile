GO ?= go
GOVULNCHECK ?= govulncheck
CMD ?=

.PHONY: test vulncheck verify-dependency-security run

test:
	$(GO) test ./...

vulncheck:
	$(GOVULNCHECK) ./...

verify-dependency-security: vulncheck

run:
	@test -n "$(CMD)" || (echo "usage: make run CMD='go test ./...'" >&2; exit 2)
	$(CMD)
