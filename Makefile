.PHONY: test unit contract build integration e2e
test: unit contract build
unit:
	cd backend && go test -race ./internal/... ./migrations/...
contract:
	cd frontend && npm run contract && npm run generate
	python3 tests/contract/check_migrations.py
build:
	cd backend && go build ./...
	cd frontend && npm run build
integration:
	cd backend && go test -v ./tests/integration/...
e2e:
	@echo 'Business E2E becomes available in D2/E2; not implemented in A1'; exit 1
