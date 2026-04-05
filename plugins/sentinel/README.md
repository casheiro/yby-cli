# 🛡️ Sentinel Plugin

Sentinel is the SRE (Site Reliability Engineering) and observability component of Yby CLI. It combines automated vulnerability detection, caching, intelligent analysis, and report generation to provide comprehensive Kubernetes security scanning and diagnostics.

## Overview

The Sentinel plugin automates security checks and analysis of Kubernetes workloads:

- **Security Scanning**: Detects common security misconfigurations in pods and containers
- **Analysis**: Uses AI to diagnose cluster issues and suggest fixes
- **Report Generation**: Exports findings in JSON, Markdown, or terminal-friendly formats
- **Caching**: Intelligently caches analysis results with TTL support

## Architecture

```
plugins/sentinel/
├── cli/                      # Main command-line interface
│   ├── main.go              # Entry point
│   ├── scan.go              # Security vulnerability detection
│   ├── cache.go             # Result caching with TTL
│   ├── report.go            # Report generation (JSON, Markdown, terminal)
│   └── *_test.go            # Unit tests (see below)
├── agent/                   # Optional AI agent implementation
└── README.md               # This file
```

## Security Checks

Sentinel performs the following security validations on Kubernetes deployments:

### Root Container Check (`checkRootContainer`)
- **Critical**: Container runs as root (UID 0)
- **Warning**: Container lacks `runAsNonRoot=true` configuration
- **Recommendation**: Always run containers as non-root user

### Resource Limits Check (`checkResourceLimits`)
- **Warning**: Container lacks CPU/memory limits
- **Why**: Unlimited containers can consume cluster resources and cause DoS
- **Fix**: Define `resources.limits` for CPU and memory

### Image Pull Policy Check (`checkImagePullPolicy`)
- **Warning**: ImagePullPolicy is not `Always`
- **Why**: Non-Always policies can use stale images if not in cache
- **Recommendation**: Use `ImagePullPolicy: Always` for production

### Exposed Secrets Check (`checkExposedSecrets`)
- **Critical**: Hardcoded credentials in environment variables
- **Detection**: Scans for common patterns (DB_PASSWORD, API_KEY, TOKEN, etc.)
- **Fix**: Use Secret objects or external secret management (ESO, Sealed Secrets)

## Usage

### Basic Security Scan

```bash
# Scan a specific pod
yby sentinel investigate my-pod -n default

# Scan entire namespace (CLI)
yby sentinel scan -n production --format=markdown --output=report.md
```

### Report Formats

Sentinel can generate reports in three formats:

1. **Terminal (Default)**: Rich colored output with findings and recommendations
2. **JSON**: Machine-readable format with structured metadata
3. **Markdown**: Human-readable format suitable for documentation

### API

Sentinel is invoked by the Yby CLI through the plugin protocol.

**Request** (via `YBY_PLUGIN_REQUEST` environment variable):
```json
{
  "hook": "command",
  "args": ["scan", "-n", "production", "--format=json"],
  "context": {
    "PROJECT_PATH": "/home/user/myapp",
    "YBY_ENV": "prod"
  }
}
```

**Response** (via stdout):
```json
{
  "data": {
    "namespace": "production",
    "findings": [
      {
        "resource": "nginx/app",
        "namespace": "production",
        "type": "critical",
        "category": "root_container",
        "description": "Container 'app' runs as root (UID 0)"
      }
    ],
    "recommendations": "Run containers as non-root users..."
  }
}
```

## Testing

### Unit Tests (v4.0+)

Sentinel includes comprehensive unit test coverage for all core modules:

**Test Files:**
- `cli/cache_test.go` - Cache behavior, TTL, round-trip serialization (6 tests)
- `cli/report_test.go` - JSON/Markdown format validation, output capture (7 tests)
- `cli/scan_test.go` - Security check functions and report generation (12 tests)

**Total:** 25 unit tests with no external dependencies

### Running Tests

```bash
# Run all unit tests
task test

# Run Sentinel tests specifically (requires k8s build tag)
go test -v -tags=k8s ./plugins/sentinel/cli/...

# Run a specific test
go test -v -run TestCheckRootContainer_RunAsRoot -tags=k8s ./plugins/sentinel/cli/...
```

### Build Tag

Tests use the `k8s` build tag to isolate Kubernetes-specific code:

```go
//go:build k8s
```

This allows tests to import `k8s.io/api/core/v1` without affecting non-Kubernetes builds.

### Test Strategy

Tests follow a **pure function** testing pattern:

1. **Extracted Pure Functions**: Security checks are pure functions with no side effects
   ```go
   func checkRootContainer(pod corev1.Pod, container corev1.Container, namespace string) []SecurityFinding
   ```

2. **Minimal Fixtures**: Tests construct only the Kubernetes objects needed to trigger specific conditions
   ```go
   container := corev1.Container{
       SecurityContext: &corev1.SecurityContext{RunAsUser: &uid},
   }
   findings := checkRootContainer(pod, container, "default")
   ```

3. **No External Dependencies**: Tests don't require a live cluster, Docker, or complex mocks

4. **Fast Execution**: Full test suite completes in ~500ms

### Testing Documentation

For detailed information on the testing implementation, see:
- **English**: [docs/sentinel-testing.md](../../docs/sentinel-testing.md)
- **Português (Brasil)**: [docs/sentinel-testes-pt.md](../../docs/sentinel-testes-pt.md)

## Configuration

Sentinel respects the following environment variables:

| Variable | Purpose | Example |
|---|---|---|
| `YBY_ENV` | Active environment context | `prod`, `staging`, `dev` |
| `YBY_AI_PROVIDER` | Force AI provider (overrides auto-detect) | `ollama`, `gemini`, `openai` |
| `YBY_AI_LANGUAGE` | Language for AI recommendations | `pt-BR`, `en-US`, `es-ES` |

## AI Integration

When findings are detected, Sentinel uses the Yby AI factory to generate contextualized recommendations:

1. **Auto-Detection**: Tries Ollama (local), then Gemini, then OpenAI
2. **Fallback**: If no AI provider is available, reports findings without recommendations
3. **Customization**: Set `YBY_AI_PROVIDER` to force a specific provider

## Performance Considerations

- **Caching**: Analysis results are cached with a 1-hour TTL to avoid redundant scans
- **Scalability**: Linear performance relative to pod count (each pod scanned once)
- **Memory**: Minimal footprint (stateless, streaming design)

## Troubleshooting

### Tests Fail with "k8s tag not found"
Ensure you're using the build tag:
```bash
go test -tags=k8s ./plugins/sentinel/cli/...
```

### Cache Not Working
Verify that the cache directory exists and is writable:
```bash
ls -la ~/.yby/
```

### AI Provider Not Detected
Check environment variables:
```bash
env | grep YBY_AI
env | grep OLLAMA_HOST
env | grep GEMINI_API_KEY
```

## Development

### Adding New Security Checks

1. Create a new `check*()` function in `scan.go` with the standard signature
2. Write unit tests in `scan_test.go`
3. Call the function from `scanNamespace()`
4. Update the `SecurityFinding` category enum if needed

Example:
```go
func checkNewVulnerability(pod corev1.Pod, container corev1.Container, namespace string) []SecurityFinding {
    var findings []SecurityFinding
    // Implementation
    return findings
}
```

### Improving Report Formats

Report generation functions are in `report.go`:
- `exportJSON()` - JSON format
- `exportMarkdown()` - Markdown format
- `renderScanResult()` - Terminal rendering

Each function is tested independently for easy iteration.

## Files Modified (feat/sentinel-unit-tests)

- `cli/scan.go` - Refactored with extracted pure functions
- `cli/cache_test.go` - New file (6 tests)
- `cli/report_test.go` - New file (7 tests)
- `cli/scan_test.go` - New file (12 tests)
- `cli/integration_test.go` - Updated with new fixtures

## References

- [Kubernetes Security Best Practices](https://kubernetes.io/docs/concepts/security/)
- [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/)
- [client-go Documentation](https://github.com/kubernetes/client-go)
- [Go Testing Guide](https://pkg.go.dev/testing)

## License

Same as Yby CLI - See project root for details.

---

**Last Updated**: 2026-04-04  
**Maintainers**: Yby Development Team
