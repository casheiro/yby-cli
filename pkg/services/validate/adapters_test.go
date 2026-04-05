package validate

import (
	"context"
	"fmt"
	"testing"

	"github.com/casheiro/yby-cli/pkg/testutil"
	"github.com/stretchr/testify/assert"
)

func TestRealHelmRunner_DependencyBuild(t *testing.T) {
	t.Run("sucesso", func(t *testing.T) {
		runner := &testutil.MockRunner{
			RunFunc: func(_ context.Context, name string, args ...string) error {
				assert.Equal(t, "helm", name)
				assert.Equal(t, []string{"dependency", "build", "charts/system"}, args)
				return nil
			},
		}
		helm := &RealHelmRunner{Runner: runner}

		err := helm.DependencyBuild(context.Background(), "charts/system")
		assert.NoError(t, err)
	})

	t.Run("erro", func(t *testing.T) {
		runner := &testutil.MockRunner{
			RunFunc: func(_ context.Context, _ string, _ ...string) error {
				return fmt.Errorf("helm dependency build falhou")
			},
		}
		helm := &RealHelmRunner{Runner: runner}

		err := helm.DependencyBuild(context.Background(), "charts/system")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "falhou")
	})
}

func TestRealHelmRunner_Lint(t *testing.T) {
	t.Run("sucesso", func(t *testing.T) {
		runner := &testutil.MockRunner{
			RunFunc: func(_ context.Context, name string, args ...string) error {
				assert.Equal(t, "helm", name)
				assert.Equal(t, []string{"lint", "charts/bootstrap"}, args)
				return nil
			},
		}
		helm := &RealHelmRunner{Runner: runner}

		err := helm.Lint(context.Background(), "charts/bootstrap")
		assert.NoError(t, err)
	})

	t.Run("erro", func(t *testing.T) {
		runner := &testutil.MockRunner{
			RunFunc: func(_ context.Context, _ string, _ ...string) error {
				return fmt.Errorf("lint falhou")
			},
		}
		helm := &RealHelmRunner{Runner: runner}

		err := helm.Lint(context.Background(), "charts/bootstrap")
		assert.Error(t, err)
	})
}

func TestRealHelmRunner_Template(t *testing.T) {
	t.Run("sucesso", func(t *testing.T) {
		runner := &testutil.MockRunner{
			RunCombinedOutputFunc: func(_ context.Context, name string, args ...string) ([]byte, error) {
				assert.Equal(t, "helm", name)
				assert.Equal(t, []string{"template", "release-name", "charts/system", "-f", "values.yaml"}, args)
				return []byte("---\napiVersion: v1"), nil
			},
		}
		helm := &RealHelmRunner{Runner: runner}

		out, err := helm.Template(context.Background(), "release-name", "charts/system", "values.yaml")
		assert.NoError(t, err)
		assert.Contains(t, string(out), "apiVersion")
	})

	t.Run("erro", func(t *testing.T) {
		runner := &testutil.MockRunner{
			RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
				return []byte("Error: rendering failed"), fmt.Errorf("exit status 1")
			},
		}
		helm := &RealHelmRunner{Runner: runner}

		out, err := helm.Template(context.Background(), "release-name", "charts/system", "values.yaml")
		assert.Error(t, err)
		assert.Contains(t, string(out), "Error")
	})
}
