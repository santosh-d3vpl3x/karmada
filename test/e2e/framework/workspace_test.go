/*
Copyright 2026 The Karmada Authors.

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

package framework

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunWorkspaceKubectlUsesConfiguredBinary(t *testing.T) {
	tempDir := t.TempDir()
	kubeconfigPath := filepath.Join(tempDir, "workspace.kubeconfig")
	if err := os.WriteFile(kubeconfigPath, []byte("apiVersion: v1\nkind: Config\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	fakeKubectlPath := filepath.Join(tempDir, "fake-kubectl")
	if err := os.WriteFile(fakeKubectlPath, []byte("#!/bin/sh\nprintf '%s\\n' \"$*\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv(WorkspaceKubectlBinEnv, fakeKubectlPath)

	output, err := RunWorkspaceKubectl(kubeconfigPath, "api-resources", "-o", "name")
	if err != nil {
		t.Fatalf("RunWorkspaceKubectl returned error: %v, output: %s", err, output)
	}
	if !strings.Contains(output, "--kubeconfig "+kubeconfigPath+" api-resources -o name") {
		t.Fatalf("unexpected kubectl invocation: %q", output)
	}
}
