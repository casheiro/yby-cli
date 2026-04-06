package cmd

import (
	"testing"

	"github.com/casheiro/yby-cli/pkg/services/setup"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/casheiro/yby-cli/pkg/testutil"
	"github.com/stretchr/testify/assert"
)

func newMockSetupService(checker setup.ToolChecker, pkg setup.PackageManager) setup.Service {
	return setup.NewService(checker, pkg, &testutil.MockRunner{}, &testutil.MockFilesystem{})
}

func TestSetupCmd_DevProfile_AllInstalled(t *testing.T) {
	original := newSetupService
	defer func() { newSetupService = original }()

	newSetupService = func(r shared.Runner, fs shared.Filesystem) setup.Service {
		checker := &testutil.MockRunner{
			LookPathFunc: func(file string) (string, error) {
				return "/usr/bin/" + file, nil
			},
		}
		return setup.NewService(
			&setup.SystemToolChecker{Runner: checker},
			&setup.SystemPackageManager{Runner: checker},
			&testutil.MockRunner{},
			&testutil.MockFilesystem{},
		)
	}

	err := setupCmd.RunE(setupCmd, []string{})
	assert.NoError(t, err)
}

func TestSetupCmd_ServerProfile(t *testing.T) {
	original := newSetupService
	defer func() { newSetupService = original }()

	newSetupService = func(r shared.Runner, fs shared.Filesystem) setup.Service {
		checker := &testutil.MockRunner{
			LookPathFunc: func(file string) (string, error) {
				return "/usr/bin/" + file, nil
			},
		}
		return setup.NewService(
			&setup.SystemToolChecker{Runner: checker},
			&setup.SystemPackageManager{Runner: checker},
			&testutil.MockRunner{},
			&testutil.MockFilesystem{},
		)
	}

	setupCmd.Flags().Set("profile", "server")
	defer setupCmd.Flags().Set("profile", "dev")

	err := setupCmd.RunE(setupCmd, []string{})
	assert.NoError(t, err)
}

func TestSetupCmd_InvalidProfile(t *testing.T) {
	setupCmd.Flags().Set("profile", "invalid")
	defer setupCmd.Flags().Set("profile", "dev")

	err := setupCmd.RunE(setupCmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Perfil inválido")
}
