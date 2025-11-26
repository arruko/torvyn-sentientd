.PHONY: help db-up db-down db-migrate db-migrate-down sqlc-generate build run test clean

# Default database connection string for local development
DB_URL ?= postgres://sentientd:sentientd@localhost:5432/sentientd?sslmode=disable

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

db-up: ## Start Postgres via Docker Compose
	docker-compose up -d postgres
	@echo "Waiting for Postgres to be ready..."
	@until docker exec sentientd-postgres pg_isready -U sentientd > /dev/null 2>&1; do sleep 1; done
	@echo "Postgres is ready!"

db-down: ## Stop Postgres
	docker-compose down

db-migrate: ## Run database migrations
	@which migrate > /dev/null || (echo "Error: golang-migrate not found. Install with: brew install golang-migrate" && exit 1)
	migrate -path migrations -database "$(DB_URL)" up

db-migrate-down: ## Rollback database migrations
	migrate -path migrations -database "$(DB_URL)" down

db-reset: ## Reset database (down + up)
	migrate -path migrations -database "$(DB_URL)" down -all || true
	migrate -path migrations -database "$(DB_URL)" up

sqlc-generate: ## Generate sqlc code
	~/go/bin/sqlc generate

build: ## Build sentientd binary
	go build -o bin/sentientd ./cmd

run: ## Run sentientd locally (requires DB_URL)
	@export DB_URL=$(DB_URL) && go run ./cmd/main.go

test: ## Run tests
	go test -v -race ./...

test-integration: db-up db-migrate ## Run integration tests with real Postgres
	@export DB_URL=$(DB_URL) && go test -v -race -tags=integration ./...
	@make db-down

clean: ## Clean build artifacts
	rm -rf bin/

dev-setup: db-up db-migrate sqlc-generate ## Setup local development environment
	@echo "Local development environment ready!"
	@echo "Database URL: $(DB_URL)"
	@echo "Run 'make run' to start sentientd"

# Kind cluster targets
dev-kind: ## Create Kind cluster with full stack
	@echo "Creating Kind cluster..."
	kind create cluster --name sentientd
	@echo "Installing Argo Workflows..."
	kubectl create namespace argo
	kubectl apply -n argo -f https://github.com/argoproj/argo-workflows/releases/download/v3.5.5/install.yaml
	@echo "Installing PostgreSQL..."
	helm repo add bitnami https://charts.bitnami.com/bitnami || true
	helm repo update
	helm install postgres bitnami/postgresql \
	  --namespace sentientd \
	  --create-namespace \
	  --set auth.username=sentientd \
	  --set auth.password=sentientd \
	  --set auth.database=sentientd \
	  --set primary.persistence.size=1Gi
	@echo "Waiting for Postgres..."
	kubectl -n sentientd wait --for=condition=ready pod -l app.kubernetes.io/name=postgresql --timeout=180s
	@echo "Installing ServiceGraph CRD..."
	kubectl apply -f crds/servicegraph.torvyn.io_servicegraphs.yaml
	@echo "Kind cluster ready!"

kind-load: ## Build and load images into Kind
	docker build -t sentientd:latest .
	docker build -t ghcr.io/arruko/torvyn-dag-runner:latest -f cmd/dag-runner/Dockerfile .
	kind load docker-image sentientd:latest --name sentientd
	kind load docker-image ghcr.io/arruko/torvyn-dag-runner:latest --name sentientd
	@echo "Images loaded into Kind cluster"

kind-deploy: ## Deploy sentientd to Kind cluster
	kubectl apply -k deploy/kubernetes
	@echo "Waiting for sentientd..."
	kubectl -n sentientd wait --for=condition=ready pod -l app.kubernetes.io/name=sentientd --timeout=120s
	@echo "sentientd deployed!"

kind-clean: ## Delete Kind cluster
	kind delete cluster --name sentientd

build-dag-runner: ## Build DAG runner binary
	go build -o bin/dag-runner ./cmd/dag-runner
