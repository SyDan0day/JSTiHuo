.PHONY: test unit contract build integration e2e
test: unit contract build
unit:
	cd backend && go test -race ./internal/... ./migrations/...
contract:
	cd frontend && npm run contract && npm run generate
	node tests/contract/generate-cases.mjs
	python3 tests/contract/check_migrations.py
build:
	cd backend && go build ./...
	cd frontend && npm run build
integration:
	cd backend && CONTRACT_VALIDATE=1 CONTRACT_REQUIRE_ALL=1 go test -race -v ./tests/integration/...
e2e:
	cd backend && go test -race -v ./tests/integration -run '^TestMockFulfillment$$' -count=1
