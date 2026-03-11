package testutil

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// DummyFileInfo implements fs.FileInfo for testing
type DummyFileInfo struct {
	name  string
	size  int64
	mode  fs.FileMode
	mod   time.Time
	isDir bool
}

func (d DummyFileInfo) Name() string       { return d.name }
func (d DummyFileInfo) Size() int64        { return d.size }
func (d DummyFileInfo) Mode() fs.FileMode  { return d.mode }
func (d DummyFileInfo) ModTime() time.Time { return d.mod }
func (d DummyFileInfo) IsDir() bool        { return d.isDir }
func (d DummyFileInfo) Sys() any           { return nil }

func TestMockRunner(t *testing.T) {
	ctx := context.Background()

	t.Run("Default behaviors", func(t *testing.T) {
		runner := &MockRunner{}
		assert.NoError(t, runner.Run(ctx, "cmd"))

		out, err := runner.RunCombinedOutput(ctx, "cmd")
		assert.NoError(t, err)
		assert.Empty(t, out)

		assert.NoError(t, runner.RunStdin(ctx, "in", "cmd"))

		out, err = runner.RunStdinOutput(ctx, "in", "cmd")
		assert.NoError(t, err)
		assert.Empty(t, out)

		path, err := runner.LookPath("cmd")
		assert.NoError(t, err)
		assert.Equal(t, "cmd", path)
	})

	t.Run("Custom behaviors", func(t *testing.T) {
		runner := &MockRunner{
			RunFunc: func(ctx context.Context, name string, args ...string) error {
				if name == "fail" {
					return errors.New("failed")
				}
				return nil
			},
			RunCombinedOutputFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				return []byte("output"), nil
			},
			LookPathFunc: func(file string) (string, error) {
				return "/usr/bin/" + file, nil
			},
		}

		assert.Error(t, runner.Run(ctx, "fail"))
		assert.NoError(t, runner.Run(ctx, "success"))

		out, err := runner.RunCombinedOutput(ctx, "cmd")
		assert.NoError(t, err)
		assert.Equal(t, "output", string(out))

		path, err := runner.LookPath("cmd")
		assert.NoError(t, err)
		assert.Equal(t, "/usr/bin/cmd", path)
	})
}

func TestMockFilesystem(t *testing.T) {
	t.Run("Default behaviors", func(t *testing.T) {
		mfs := &MockFilesystem{}

		data, err := mfs.ReadFile("test.txt")
		assert.ErrorIs(t, err, os.ErrNotExist)
		assert.Nil(t, data)

		assert.NoError(t, mfs.WriteFile("test.txt", []byte("data"), 0644))
		assert.NoError(t, mfs.MkdirAll("/a/b", 0755))

		info, err := mfs.Stat("test.txt")
		assert.ErrorIs(t, err, os.ErrNotExist)
		assert.Nil(t, info)

		home, err := mfs.UserHomeDir()
		assert.NoError(t, err)
		assert.Equal(t, "/mock/home", home)

		assert.NoError(t, mfs.WalkDir("/", nil))
	})

	t.Run("Custom behaviors", func(t *testing.T) {
		expectedData := []byte("hello world")
		dummyInfo := DummyFileInfo{name: "test.txt", size: 100, isDir: false}

		mfs := &MockFilesystem{
			ReadFileFunc: func(name string) ([]byte, error) {
				return expectedData, nil
			},
			StatFunc: func(name string) (fs.FileInfo, error) {
				return dummyInfo, nil
			},
		}

		data, err := mfs.ReadFile("file")
		assert.NoError(t, err)
		assert.Equal(t, expectedData, data)

		info, err := mfs.Stat("test.txt")
		assert.NoError(t, err)
		assert.Equal(t, dummyInfo, info)
	})
}
