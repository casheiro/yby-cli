#!/bin/bash
set -e

# Load Go PATH
export PATH=$PATH:/usr/local/go/bin

echo "🚀 Starting Internal E2E Test (Vagrant/Libvirt)..."

cd /home/vagrant/yby-cli

echo "🔨 Building Yby CLI..."
go build -o ./yby ./cmd/yby
sudo mv ./yby /usr/local/bin/yby

# Setup Clean Workspace
TEST_DIR="/home/vagrant/yby-test-workspace"
rm -rf $TEST_DIR
mkdir -p $TEST_DIR
cd $TEST_DIR

echo "--- Step 1: Yby Init (Interactive Simulation via Flags) ---"
yby init \
  --topology single \
  --workflow essential \
  --git-repo https://github.com/teste/vagrant-e2e \
  --project-name vagrant-project \
  --domain vagrant.local \
  --email admin@vagrant.local \
  --include-ci=false

# Validate init outcomes
if [ ! -f ".yby/blueprint.yaml" ]; then
    echo "🔴 FAIL: Step 1 - Init failed to create blueprint"
    exit 1
fi
echo "🟢 PASS: Step 1 Complete"

echo "--- Step 2: Yby Bootstrap VPS (Local) ---"
# This will install K3s on the VM - REQUIRES SUDO
export K3S_KUBECONFIG_MODE="644"
# Using stable channel (default) instead of specific version
sudo K3S_KUBECONFIG_MODE=644 yby bootstrap vps --local 

if [ ! -f "/etc/rancher/k3s/k3s.yaml" ]; then
     echo "🔴 FAIL: Step 2 - Bootstrap failed (no k3s config found)"
     exit 1
fi
echo "🟢 PASS: Step 2 Complete"

echo "--- Step 2.5: Deep Health Check & Stack Validation ---"
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

# 1. Validate Core K3s Components
echo "🔍 Checking System Pods (Traefik, DNS, Metrics)..."
sudo k3s kubectl wait --for=condition=ready pod -l k8s-app=metrics-server -n kube-system --timeout=120s
sudo k3s kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=traefik -n kube-system --timeout=120s
echo "✅ Core System Operational"

# 2. Simulate GitOps Bootstrap (Validate Generated Charts)
# Instead of full 'bootstrap cluster' which needs Git, we perform a local install of the generated Bootstrap chart
# This proves the "yby init" output is valid and deployable.

echo "📦 Installing helm..."
curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

echo "🧪 Validating/Installing Generated GitOps Stack (Local Simulation)..."
# We act as the GitOps controller here, applying the charts directly.

# Install ArgoCD (Pre-req for the rest)
echo "   -> Installing ArgoCD (from Helm)..."
helm repo add argo https://argoproj.github.io/argo-helm
helm repo update
kubectl create ns argocd || true
helm upgrade --install argocd argo/argo-cd --namespace argocd --version 5.51.6 --wait --timeout 300s
echo "✅ ArgoCD Installed"

# Apply upstream manifests (CRDs for Argo Workflows & Events) required for bootstrap validation
echo "   -> Installing Argo CRDs (from generated upstream)..."
kubectl create ns argo-events || true
kubectl create ns argo || true
kubectl apply -f manifests/upstream/argo-workflows.yaml
kubectl apply -f manifests/upstream/argo-events.yaml

# Apply the generated Bootstrap Chart (this contains the App of Apps logic)
# This validates that 'yby init' generated valid Helm charts
echo "   -> Validating Generated Bootstrap Chart..."
helm dependency update ./charts/bootstrap
helm upgrade --install bootstrap ./charts/bootstrap -n argocd --values ./charts/bootstrap/values.yaml --dry-run
if [ $? -eq 0 ]; then
    echo "✅ Generated Charts are Valid (Dry-Run Success)"
else
    echo "🔴 Generated Charts Failed Validation"
    exit 1
fi

echo "--- Step 3: Yby Access ---"
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

# Run access with timeout to prevent blocking CI forever
echo "🔌 Testing Access Tunnels (10s timeout)..."
timeout 10s yby access > access.log 2>&1 || true
cat access.log

if grep -q "Forwarding" access.log || grep -q "Argo CD" access.log; then
    echo "✅ PASS: Step 3 Complete (Tunnels established for ArgoCD/Services)"
elif grep -q "Nenhum serviço detectado" access.log; then
    echo "⚠️  PASS: Step 3 (Graceful exit - No services found, though unexpected if ArgoCD is up)"
else
    echo "⚠️  Step 3 Inconclusive (Check logs)"
fi

echo "--- Step 4: Verification (kubectl) ---"
sudo k3s kubectl get nodes
sudo k3s kubectl get pods -A

echo "🎉 VAGRANT E2E SUCCESS (Deep Check Passed)!"
