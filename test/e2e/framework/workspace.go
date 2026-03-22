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
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

const workspaceProxyURLFormat = "%s/apis/workspace.karmada.io/v1alpha1/workspaces/%s/proxy/"

// WorkspaceKubectlBinEnv overrides the kubectl binary used by workspace smoke helpers.
const WorkspaceKubectlBinEnv = "WORKSPACE_KUBECTL_BIN"

// WorkspaceAccess captures the minimum files and endpoint shape needed for workspace e2e.
type WorkspaceAccess struct {
	Name           string
	KubeconfigPath string
	Endpoint       string
}

// BuildWorkspaceProxyURL returns the phase-1 workspace proxy URL rooted at the Karmada front door.
func BuildWorkspaceProxyURL(karmadaHost, workspaceName string) string {
	return fmt.Sprintf(workspaceProxyURLFormat, karmadaHost, workspaceName)
}

// WriteWorkspaceKubeconfig persists a generated workspace kubeconfig into a temporary test directory.
func WriteWorkspaceKubeconfig(dir, workspaceName string, kubeconfig []byte) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("%s.kubeconfig", workspaceName))
	if err := os.WriteFile(path, kubeconfig, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// LoadWorkspaceRESTClientConfig loads a rest.Config from the generated workspace kubeconfig.
func LoadWorkspaceRESTClientConfig(kubeconfigPath string) (*rest.Config, error) {
	return LoadRESTClientConfig(kubeconfigPath, "")
}

// RunWorkspaceKubectl runs kubectl against the generated workspace kubeconfig.
func RunWorkspaceKubectl(kubeconfigPath string, args ...string) (string, error) {
	return RunWorkspaceKubectlWithInput(kubeconfigPath, nil, args...)
}

// RunWorkspaceKubectlWithInput runs kubectl against the generated workspace kubeconfig with optional stdin.
func RunWorkspaceKubectlWithInput(kubeconfigPath string, stdin io.Reader, args ...string) (string, error) {
	cmd := NewWorkspaceKubectlCommand(nil, kubeconfigPath, args...)
	cmd.Stdin = stdin
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// WorkspaceKubectlBinary returns the kubectl binary used by workspace smoke helpers.
func WorkspaceKubectlBinary() string {
	if bin := os.Getenv(WorkspaceKubectlBinEnv); bin != "" {
		return bin
	}
	return "kubectl"
}

// LookupWorkspaceKubectl resolves the kubectl binary used by workspace smoke helpers.
func LookupWorkspaceKubectl() (string, error) {
	return exec.LookPath(WorkspaceKubectlBinary())
}

// NewWorkspaceKubectlCommand returns a kubectl command for the generated workspace kubeconfig.
func NewWorkspaceKubectlCommand(ctx context.Context, kubeconfigPath string, args ...string) *exec.Cmd {
	cmdArgs := append([]string{"--kubeconfig", kubeconfigPath}, args...)
	kubectlBin := WorkspaceKubectlBinary()
	if ctx != nil {
		return exec.CommandContext(ctx, kubectlBin, cmdArgs...) //nolint:gosec
	}
	return exec.Command(kubectlBin, cmdArgs...) //nolint:gosec
}

// RunWorkspaceKarmadactl runs karmadactl against the control-plane kubeconfig.
func RunWorkspaceKarmadactl(kubeconfigPath, karmadaContext, karmadactlPath string, timeout time.Duration, args ...string) (string, error) {
	return NewKarmadactlCommand(kubeconfigPath, karmadaContext, karmadactlPath, "", timeout, args...).ExecOrDie()
}

// GenerateWorkspaceAccess builds a workspace kubeconfig through karmadactl and returns the resulting access details.
func GenerateWorkspaceAccess(dir, workspaceName, kubeconfigPath, karmadaContext, karmadactlPath string, timeout time.Duration) (WorkspaceAccess, error) {
	kubeconfig, err := RunWorkspaceKarmadactl(kubeconfigPath, karmadaContext, karmadactlPath, timeout, "workspace", "kubeconfig", workspaceName)
	if err != nil {
		return WorkspaceAccess{}, err
	}

	path, err := WriteWorkspaceKubeconfig(dir, workspaceName, []byte(kubeconfig))
	if err != nil {
		return WorkspaceAccess{}, err
	}

	config, err := LoadWorkspaceRESTClientConfig(path)
	if err != nil {
		return WorkspaceAccess{}, err
	}

	return WorkspaceAccess{
		Name:           workspaceName,
		KubeconfigPath: path,
		Endpoint:       config.Host,
	}, nil
}

// NewWorkspaceAccess records the generated kubeconfig path and workspace endpoint for one test workspace.
func NewWorkspaceAccess(name, kubeconfigPath, karmadaHost string) WorkspaceAccess {
	return WorkspaceAccess{
		Name:           name,
		KubeconfigPath: kubeconfigPath,
		Endpoint:       BuildWorkspaceProxyURL(karmadaHost, name),
	}
}

// AssertWorkspaceObjectLabel checks the projected source-object ownership label used by workspace writes.
func AssertWorkspaceObjectLabel(obj metav1.Object, workspaceName string) error {
	if obj.GetLabels()["workspace.karmada.io/name"] != workspaceName {
		return fmt.Errorf("expected workspace label %q on %s/%s", workspaceName, obj.GetNamespace(), obj.GetName())
	}
	return nil
}

// WorkspaceCommandTimeout is the default timeout used by minimal workspace e2e command helpers.
const WorkspaceCommandTimeout = 2 * time.Minute
