#!/bin/bash
set -e

# Setup Environment
export PATH=$PATH:/usr/local/go/bin
APP_DIR="/home/vagrant/yby-cli"
TEST_ROOT="/home/vagrant/adv_validation"
BINARY="/usr/local/bin/yby"
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

function fail() {
    echo -e "${RED}FAILED: $1${NC}"
    exit 1
}

function pass() {
    echo -e "${GREEN}PASSED: $1${NC}"
}

echo "🔨 Building Yby CLI..."
cd $APP_DIR
go build -o yby ./cmd/yby
sudo mv yby $BINARY
cd ~

echo "🚀 Starting Advanced Validation..."

# Clean previous runs
rm -rf $TEST_ROOT
mkdir -p $TEST_ROOT

# ==============================================================================
# P1: Dev Local without GITHUB_REPO/TOKEN
# Config: Environment=local, Topology=single
# Expectation: Init success, Blueprint generated without GH token prompts/errors
# ==============================================================================
echo -e "\n--- [P1] Testing Dev Local w/o GITHUB_REPO ---"
mkdir -p $TEST_ROOT/p1_local
cd $TEST_ROOT/p1_local

# Unset GH variables to force failure if they are required
unset GITHUB_REPO
unset GITHUB_TOKEN

# Using interactive simulation flags
yby init \
  --topology single \
  --workflow essential \
  --git-repo "git://local/mirror.git" \
  --project-name "p1-project" \
  --domain "local.dev" \
  --include-ci=false \
  --env local

if [ ! -f ".yby/blueprint.yaml" ]; then
    fail "[P1] Blueprint not found after init"
else
    pass "[P1] Init completed without GITHUB_REPO/TOKEN"
fi

# ==============================================================================
# P2: .github Folder Location (TargetDir checking)
# Expectation: .github goes to Root, not inside TargetDir
# ==============================================================================
echo -e "\n--- [P2] Testing .github Placement (TargetDir) ---"
mkdir -p $TEST_ROOT/p2_repo
cd $TEST_ROOT/p2_repo
git init -q # Emulate git root

yby init \
  --target-dir infra \
  --topology single \
  --workflow essential \
  --git-repo "http://git.fake/repo" \
  --project-name "p2-project" \
  --domain "p2.dev" \
  --env dev

if [ -d "infra/.github" ]; then
    fail "[P2] .github folder found INSIDE infra/ (Should be in root)"
fi

if [ ! -d ".github" ]; then
    fail "[P2] .github folder NOT found in root"
else
    pass "[P2] .github correctly placed in root"
fi

# ==============================================================================
# P3: environments.yaml Coherence
# Expectation: 'current' matches env var, values file exists
# ==============================================================================
echo -e "\n--- [P3] Testing Environments Consistency ---"
mkdir -p $TEST_ROOT/p3_env
cd $TEST_ROOT/p3_env

yby init \
  --topology complete \
  --env dev \
  --workflow essential \
  --git-repo "http://git.fake/repo" \
  --project-name "p3-project" \
  --domain "p3.dev"

# Check environments.yaml content (simple grep check)
if grep -q "current: dev" .yby/environments.yaml; then
   pass "[P3] Current env is 'dev'"
else
   fail "[P3] Current env mismatch in environments.yaml"
fi

if [ ! -f "config/values-dev.yaml" ]; then
    fail "[P3] config/values-dev.yaml missing"
else
    pass "[P3] values-dev.yaml generated"
fi

# ==============================================================================
# P4: No .env Dependency
# Expectation: bootstrap vps runs purely on flags
# ==============================================================================
echo -e "\n--- [P4] Testing No .env Dependency ---"
mkdir -p $TEST_ROOT/p4_noenv
cd $TEST_ROOT/p4_noenv

# Ensure no .env
rm -f .env

# We expect this to FAIL connectivity (no real SSH), but NOT fail on "missing .env file"
# We adhere to the output message analysis
OUTPUT=$(yby bootstrap vps --local --host 127.0.0.1 --user vagrant --ssh-key /dev/null 2>&1 || true)

if echo "$OUTPUT" | grep -q "carregar arquivo .env"; then
    fail "[P4] Command tried to load .env"
elif echo "$OUTPUT" | grep -q "conexão SSH"; then # Expected failure point
    pass "[P4] Command attempted SSH without asking for .env"
elif echo "$OUTPUT" | grep -q "sucesso"; then
    pass "[P4] Command succeeded (unexpected but acceptable)"
else
    # fallback: checks if it didn't complain about .env
    pass "[P4] No .env complaint found in output"
fi

# ==============================================================================
# P5: Subdirectory Execution (Infra Root Detection)
# Expectation: Running from root detects infra/.yby
# ==============================================================================
echo -e "\n--- [P5] Testing Infra Root Detection ---"
mkdir -p $TEST_ROOT/p5_monorepo
cd $TEST_ROOT/p5_monorepo

# Create infra structure manually/via init
yby init --target-dir infra --topology complete --workflow essential --git-repo "http://git.fake/repo" --project-name p5 --domain p5.dev --env dev > /dev/null

# Go back to root
cd $TEST_ROOT/p5_monorepo

# Run a read-only command that needs context
OUTPUT_P5=$(yby env show 2>&1 || true)

if echo "$OUTPUT_P5" | grep -q "dev"; then
    pass "[P5] Detected 'dev' environment from root"
else
    echo "Output was: $OUTPUT_P5"
    fail "[P5] Failed to detect environment from root (infrastructure in infra/)"
fi

echo -e "\n🎉 ALL ADVANCED CHECKS PASSED!"
