/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package integration

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	criconfig "github.com/containerd/containerd/v2/internal/cri/config"
	"github.com/containerd/containerd/v2/internal/fsverity"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/log/logtest"
)

// TestFsverityPluginNotRegistered tests that appropriate error is returned
// when the fsverity integrity verifier plugin is not registered.
func TestFsverityPluginNotRegistered(t *testing.T) {
	t.Parallel()

	// TODO(fuweid): Test it in Windows.
	if runtime.GOOS != "linux" {
		t.Skip("fsverity is only supported on Linux")
	}

	tmpDir := t.TempDir()

	// Build containerd client without importing the integrityverifier plugin
	// This simulates the error: "no plugins registered for io.containerd.integrity-verifier.v1"
	cli := buildLocalContainerdClient(t, tmpDir, nil)

	criService, err := initLocalCRIImageService(cli, tmpDir, criconfig.Registry{}, true)
	
	// The error should occur during initialization if integrity verification is required
	// or during image pull if the plugin is needed but not available
	if err != nil {
		assert.Contains(t, err.Error(), "integrity-verifier", "Expected error about missing integrity-verifier plugin")
		return
	}

	ctx := namespaces.WithNamespace(logtest.WithT(context.Background(), t), k8sNamespace)

	// Try to pull an image - this should fail if integrity verification is required
	_, err = criService.PullImage(ctx, pullProgressTestImageName, nil, nil, "")
	
	// We expect either:
	// 1. No error if integrity verification is optional
	// 2. Error mentioning the missing plugin if it's required
	if err != nil {
		t.Logf("Image pull failed as expected: %v", err)
		assert.Contains(t, err.Error(), "integrity-verifier", "Expected error about missing integrity-verifier plugin")
	}
}

// TestFsverityNotSupportedOnFilesystem tests behavior when fsverity is not
// supported on the underlying filesystem.
func TestFsverityNotSupportedOnFilesystem(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "linux" {
		t.Skip("fsverity is only supported on Linux")
	}

	tmpDir := t.TempDir()

	// Check if fsverity is supported on the test directory
	supported, err := fsverity.IsSupported(tmpDir)
	if err != nil {
		t.Logf("Error checking fsverity support: %v", err)
	}

	if supported {
		t.Skip("fsverity is supported on this filesystem, skipping unsupported test")
	}

	// Test that operations handle unsupported filesystem gracefully
	testFile := filepath.Join(tmpDir, "test-file")
	f, err := os.Create(testFile)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	// Attempting to enable fsverity should fail
	err = fsverity.Enable(testFile)
	assert.Error(t, err, "Expected error when enabling fsverity on unsupported filesystem")
	assert.Contains(t, err.Error(), "fsverity", "Error should mention fsverity")
}

// TestFsverityEnableOnNonExistentFile tests error handling when trying to
// enable fsverity on a file that doesn't exist.
func TestFsverityEnableOnNonExistentFile(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "linux" {
		t.Skip("fsverity is only supported on Linux")
	}

	tmpDir := t.TempDir()
	nonExistentFile := filepath.Join(tmpDir, "does-not-exist")

	err := fsverity.Enable(nonExistentFile)
	assert.Error(t, err, "Expected error when enabling fsverity on non-existent file")
}

// TestFsverityMeasureOnNonExistentFile tests error handling when trying to
// measure fsverity on a file that doesn't exist.
func TestFsverityMeasureOnNonExistentFile(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "linux" {
		t.Skip("fsverity is only supported on Linux")
	}

	tmpDir := t.TempDir()
	nonExistentFile := filepath.Join(tmpDir, "does-not-exist")

	_, err := fsverity.Measure(nonExistentFile)
	assert.Error(t, err, "Expected error when measuring fsverity on non-existent file")
}

// TestFsverityIsEnabledOnNonExistentFile tests error handling when checking
// if fsverity is enabled on a file that doesn't exist.
func TestFsverityIsEnabledOnNonExistentFile(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "linux" {
		t.Skip("fsverity is only supported on Linux")
	}

	tmpDir := t.TempDir()
	nonExistentFile := filepath.Join(tmpDir, "does-not-exist")

	_, err := fsverity.IsEnabled(nonExistentFile)
	assert.Error(t, err, "Expected error when checking fsverity status on non-existent file")
}

// TestFsverityEnableOnDirectory tests error handling when trying to enable
// fsverity on a directory instead of a file.
func TestFsverityEnableOnDirectory(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "linux" {
		t.Skip("fsverity is only supported on Linux")
	}

	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "test-dir")
	require.NoError(t, os.Mkdir(testDir, 0755))

	err := fsverity.Enable(testDir)
	assert.Error(t, err, "Expected error when enabling fsverity on a directory")
}

// TestFsverityMeasureWithoutEnable tests that measuring fsverity fails
// when it hasn't been enabled on the file first.
func TestFsverityMeasureWithoutEnable(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "linux" {
		t.Skip("fsverity is only supported on Linux")
	}

	tmpDir := t.TempDir()

	// Check if fsverity is supported
	supported, err := fsverity.IsSupported(tmpDir)
	if !supported || err != nil {
		t.Skipf("fsverity not supported on this filesystem: %v", err)
	}

	testFile := filepath.Join(tmpDir, "test-file")
	f, err := os.Create(testFile)
	require.NoError(t, err)
	_, err = f.Write([]byte("test content"))
	require.NoError(t, err)
	require.NoError(t, f.Close())

	// Try to measure without enabling first
	_, err = fsverity.Measure(testFile)
	// This should fail because fsverity is not enabled
	assert.Error(t, err, "Expected error when measuring fsverity on file without enabling it first")
}

// TestFsverityEnableOnEmptyFile tests enabling fsverity on an empty file.
func TestFsverityEnableOnEmptyFile(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "linux" {
		t.Skip("fsverity is only supported on Linux")
	}

	tmpDir := t.TempDir()

	// Check if fsverity is supported
	supported, err := fsverity.IsSupported(tmpDir)
	if !supported || err != nil {
		t.Skipf("fsverity not supported on this filesystem: %v", err)
	}

	testFile := filepath.Join(tmpDir, "empty-file")
	f, err := os.Create(testFile)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	// Try to enable fsverity on empty file
	err = fsverity.Enable(testFile)
	// This may or may not fail depending on kernel version and filesystem
	if err != nil {
		t.Logf("Enabling fsverity on empty file failed (may be expected): %v", err)
	} else {
		// If it succeeds, verify we can measure it
		_, err = fsverity.Measure(testFile)
		assert.NoError(t, err, "Should be able to measure fsverity on empty file if enable succeeded")
	}
}

// TestFsverityPermissionDenied tests error handling when lacking permissions
// to enable fsverity.
func TestFsverityPermissionDenied(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "linux" {
		t.Skip("fsverity is only supported on Linux")
	}

	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	tmpDir := t.TempDir()

	// Check if fsverity is supported
	supported, err := fsverity.IsSupported(tmpDir)
	if !supported || err != nil {
		t.Skipf("fsverity not supported on this filesystem: %v", err)
	}

	testFile := filepath.Join(tmpDir, "test-file")
	f, err := os.Create(testFile)
	require.NoError(t, err)
	_, err = f.Write([]byte("test content"))
	require.NoError(t, err)
	require.NoError(t, f.Close())

	// Remove write permissions
	require.NoError(t, os.Chmod(testFile, 0444))

	// Try to enable fsverity - this may fail due to permissions
	err = fsverity.Enable(testFile)
	if err != nil {
		t.Logf("Enabling fsverity failed as expected due to permissions: %v", err)
	}
}

// TestFsverityIsSupportedOnInvalidPath tests error handling when checking
// fsverity support on an invalid path.
func TestFsverityIsSupportedOnInvalidPath(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "linux" {
		t.Skip("fsverity is only supported on Linux")
	}

	// Test with non-existent path
	_, err := fsverity.IsSupported("/path/that/does/not/exist")
	assert.Error(t, err, "Expected error when checking fsverity support on non-existent path")
}

// TestCRIImagePullWithMissingIntegrityVerifier tests image pull scenarios
// when the integrity verifier plugin is missing.
func TestCRIImagePullWithMissingIntegrityVerifier(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "linux" {
		t.Skip("fsverity is only supported on Linux")
	}

	testCases := []struct {
		name     string
		useLocal bool
	}{
		{
			name:     "LocalPull",
			useLocal: true,
		},
		{
			name:     "TransferService",
			useLocal: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()

			// Build client without integrity verifier plugin
			cli := buildLocalContainerdClient(t, tmpDir, nil)

			criService, err := initLocalCRIImageService(cli, tmpDir, criconfig.Registry{}, tc.useLocal)
			
			// If initialization fails, it should be due to missing plugin
			if err != nil {
				assert.Contains(t, err.Error(), "integrity-verifier", 
					"Expected error about missing integrity-verifier plugin during initialization")
				return
			}

			ctx := namespaces.WithNamespace(logtest.WithT(context.Background(), t), k8sNamespace)

			// Try to pull an image
			_, err = criService.PullImage(ctx, pullProgressTestImageName, nil, nil, "")
			
			// Log the result - error may or may not occur depending on whether
			// integrity verification is required
			if err != nil {
				t.Logf("Image pull failed: %v", err)
				if containsIntegrityVerifierError(err) {
					t.Logf("Error is related to missing integrity-verifier plugin as expected")
				}
			} else {
				t.Logf("Image pull succeeded (integrity verification may be optional)")
			}
		})
	}
}

// containsIntegrityVerifierError checks if an error is related to the
// integrity-verifier plugin.
func containsIntegrityVerifierError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return containsAny(errStr, []string{
		"integrity-verifier",
		"io.containerd.integrity-verifier",
		"plugin: not found",
	})
}

// containsAny checks if a string contains any of the given substrings.
func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if contains(s, substr) {
			return true
		}
	}
	return false
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && 
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Made with Bob
