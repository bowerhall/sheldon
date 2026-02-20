# Sheldon Makefile

.PHONY: build build-sheldon build-claude-code deploy deploy-lite deploy-minimal deploy-full clean logs test

# Default overlay for local development
OVERLAY ?= lite

# Build all container images
build: build-sheldon build-claude-code

# Build main sheldon image
build-sheldon:
	@echo "Building sheldon image..."
	docker build -t ghcr.io/bowerhall/sheldon:latest -f core/Dockerfile .

# Build ephemeral claude-code image
build-claude-code:
	@echo "Building sheldon-claude-code image..."
	docker build -t ghcr.io/bowerhall/sheldon-claude-code:latest deploy/docker/claude-code/

# Push images to ghcr.io
push: push-sheldon push-claude-code

push-sheldon:
	docker push ghcr.io/bowerhall/sheldon:latest

push-claude-code:
	docker push ghcr.io/bowerhall/sheldon-claude-code:latest

# Deploy to local k8s cluster
deploy:
	@echo "Deploying with $(OVERLAY) overlay..."
	kubectl apply -k deploy/k8s/overlays/$(OVERLAY)
	@echo "Waiting for pods to be ready..."
	kubectl wait --for=condition=ready pod -l app=sheldon -n sheldon --timeout=120s || true
	@echo ""
	@echo "Deployment status:"
	kubectl get pods -n sheldon

deploy-lite:
	$(MAKE) deploy OVERLAY=lite

deploy-minimal:
	$(MAKE) deploy OVERLAY=minimal

deploy-full:
	$(MAKE) deploy OVERLAY=full

# Restart sheldon deployment (after rebuild)
restart:
	kubectl rollout restart deployment/sheldon -n sheldon
	kubectl rollout status deployment/sheldon -n sheldon

# View logs
logs:
	kubectl logs -f deployment/sheldon -n sheldon

# Follow logs with timestamps
logs-ts:
	kubectl logs -f deployment/sheldon -n sheldon --timestamps

# Get shell in sheldon pod
shell:
	kubectl exec -it deployment/sheldon -n sheldon -- /bin/sh

# Check status
status:
	@echo "=== Namespaces ==="
	kubectl get ns | grep sheldon
	@echo ""
	@echo "=== Pods ==="
	kubectl get pods -n sheldon
	@echo ""
	@echo "=== Pods in sheldon-apps ==="
	kubectl get pods -n sheldon-apps 2>/dev/null || echo "No pods in sheldon-apps"
	@echo ""
	@echo "=== PVCs ==="
	kubectl get pvc -n sheldon

# Clean up deployment
clean:
	kubectl delete -k deploy/k8s/overlays/$(OVERLAY) --ignore-not-found
	kubectl delete namespace sheldon-apps --ignore-not-found

# Run Go tests
test:
	cd core && go test ./...

# Build and deploy (full cycle)
all: build deploy

# Local development without k8s
run-local:
	cd core && go run ./cmd/sheldon

# Check what would be deployed (dry run)
dry-run:
	kubectl apply -k deploy/k8s/overlays/$(OVERLAY) --dry-run=client

# Show kustomize output
show:
	kubectl kustomize deploy/k8s/overlays/$(OVERLAY)
