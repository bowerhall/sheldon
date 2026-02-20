# Kora Makefile

.PHONY: build build-kora build-claude-code deploy deploy-lite deploy-minimal deploy-full clean logs test

# Default overlay for local development
OVERLAY ?= lite

# Build all container images
build: build-kora build-claude-code

# Build main kora image
build-kora:
	@echo "Building kora image..."
	docker build -t kora:latest -f core/Dockerfile .

# Build ephemeral claude-code image
build-claude-code:
	@echo "Building kora-claude-code image..."
	docker build -t kora-claude-code:latest deploy/docker/claude-code/

# Deploy to local k8s cluster
deploy:
	@echo "Deploying with $(OVERLAY) overlay..."
	kubectl apply -k deploy/k8s/overlays/$(OVERLAY)
	@echo "Waiting for pods to be ready..."
	kubectl wait --for=condition=ready pod -l app=kora -n kora --timeout=120s || true
	@echo ""
	@echo "Deployment status:"
	kubectl get pods -n kora

deploy-lite:
	$(MAKE) deploy OVERLAY=lite

deploy-minimal:
	$(MAKE) deploy OVERLAY=minimal

deploy-full:
	$(MAKE) deploy OVERLAY=full

# Restart kora deployment (after rebuild)
restart:
	kubectl rollout restart deployment/kora -n kora
	kubectl rollout status deployment/kora -n kora

# View logs
logs:
	kubectl logs -f deployment/kora -n kora

# Follow logs with timestamps
logs-ts:
	kubectl logs -f deployment/kora -n kora --timestamps

# Get shell in kora pod
shell:
	kubectl exec -it deployment/kora -n kora -- /bin/sh

# Check status
status:
	@echo "=== Namespaces ==="
	kubectl get ns | grep kora
	@echo ""
	@echo "=== Pods ==="
	kubectl get pods -n kora
	@echo ""
	@echo "=== Pods in kora-apps ==="
	kubectl get pods -n kora-apps 2>/dev/null || echo "No pods in kora-apps"
	@echo ""
	@echo "=== PVCs ==="
	kubectl get pvc -n kora

# Clean up deployment
clean:
	kubectl delete -k deploy/k8s/overlays/$(OVERLAY) --ignore-not-found
	kubectl delete namespace kora-apps --ignore-not-found

# Run Go tests
test:
	cd core && go test ./...

# Build and deploy (full cycle)
all: build deploy

# Local development without k8s
run-local:
	cd core && go run ./cmd/kora

# Check what would be deployed (dry run)
dry-run:
	kubectl apply -k deploy/k8s/overlays/$(OVERLAY) --dry-run=client

# Show kustomize output
show:
	kubectl kustomize deploy/k8s/overlays/$(OVERLAY)
