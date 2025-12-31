#!/bin/bash
set -e

# Unique name to avoid conflicts
CONTAINER_NAME="yby-safe-verify-$(date +%s)"

echo "🛡️  Starting ISOLATED container environment..."
# Using golang:alpine (latest) to support Go 1.24+ deps
docker run -d --name $CONTAINER_NAME -w /app golang:alpine tail -f /dev/null > /dev/null

echo "📦 Copying source code to container..."
# Copy current dir contents to /app in container
docker cp . $CONTAINER_NAME:/app

echo "🔨 Building binary inside container..."
docker exec $CONTAINER_NAME go build -o /usr/local/bin/yby ./cmd/yby

echo "🚀 Running 'yby init' inside container (isolated workspace)..."
# Create a clean subdir for the init execution
docker exec $CONTAINER_NAME mkdir -p /tmp/test-run
# Execute init
docker exec -w /tmp/test-run $CONTAINER_NAME yby init \
  --topology complete \
  --workflow gitflow \
  --git-repo https://github.com/teste/iso-test \
  --project-name isolated-project \
  --domain isolated.local \
  --email admin@isolated.local \
  --enable-kepler \
  --enable-minio \
  --enable-keda \
  --include-devcontainer

echo "✅ Verifying artifacts inside container..."

fail=0

check_file_content() {
    file=$1
    pattern=$2
    desc=$3
    
    if docker exec -w /tmp/test-run $CONTAINER_NAME grep -q "$pattern" "$file"; then
        echo "🟢 PASS: $desc"
    else
        echo "🔴 FAIL: $desc (Pattern '$pattern' not found in $file)"
        # Debug output
        docker exec -w /tmp/test-run $CONTAINER_NAME cat "$file"
        fail=1
    fi
}

echo "--- Checking config/cluster-values.yaml ---"
check_file_content "config/cluster-values.yaml" "domainBase: \"isolated.local\"" "Domain Base"
check_file_content "config/cluster-values.yaml" "email: admin@isolated.local" "Email"
check_file_content "config/cluster-values.yaml" "repoName: isolated-project" "Project Name"

# Check modules enabled (context-based logic)
check_file_content "config/cluster-values.yaml" "enabled: true" "Kepler/MinIO/KEDA (heuristic)"

# Verify Blueprint existence
if docker exec -w /tmp/test-run $CONTAINER_NAME [ -f .yby/blueprint.yaml ]; then
    echo "🟢 PASS: Blueprint generated"
else
    echo "🔴 FAIL: Blueprint missing"
    fail=1
fi

echo "🧹 Cleaning up container..."
docker rm -f $CONTAINER_NAME > /dev/null

if [ $fail -eq 0 ]; then
    echo "🎉 SAFE VALIDATION SUCCESS!"
    exit 0
else
    echo "❌ VALIDATION FAILED"
    exit 1
fi
