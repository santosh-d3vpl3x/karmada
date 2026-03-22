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

package hack

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeployWorkspaceApiserverReusesExistingCertsAndRendersConfiguredImage(t *testing.T) {
	repoDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	tempDir := t.TempDir()
	fakeBinDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(fakeBinDir, 0o755); err != nil {
		t.Fatal(err)
	}

	homeDir := filepath.Join(tempDir, "home")
	certDir := filepath.Join(homeDir, ".karmada")
	if err := os.MkdirAll(certDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"ca.crt",
		"ca.key",
		"ca-config.json",
		"karmada-workspace-apiserver.crt",
		"karmada-workspace-apiserver.key",
		"karmada-workspace-apiserver-client.crt",
		"karmada-workspace-apiserver-client.key",
		"karmada-workspace-apiserver-etcd-client.crt",
		"karmada-workspace-apiserver-etcd-client.key",
	} {
		if err := os.WriteFile(filepath.Join(certDir, name), []byte(name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	hostKubeconfig := filepath.Join(tempDir, "host.kubeconfig")
	karmadaKubeconfig := filepath.Join(tempDir, "karmada.kubeconfig")
	for _, path := range []string{hostKubeconfig, karmadaKubeconfig} {
		if err := os.WriteFile(path, []byte("apiVersion: v1\nkind: Config\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	kubectlLog := filepath.Join(tempDir, "kubectl.log")
	renderedDeployment := filepath.Join(tempDir, "rendered-workspace-apiserver.yaml")
	kubectlScript := `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$KUBECTL_LOG"
if [ "${1:-}" = "config" ] && [ "${2:-}" = "get-contexts" ]; then
  case "${3:-}" in
    karmada-host|karmada-apiserver) exit 0 ;;
    *) exit 1 ;;
  esac
fi
prev=""
file=""
for arg in "$@"; do
  if [ "$prev" = "-f" ]; then
    file="$arg"
  fi
  prev="$arg"
done
case " $* " in
  *" apply -f "*)
    if [ -n "$file" ] && [ -n "${KUBECTL_CAPTURE_DEPLOYMENT:-}" ] && echo "$file" | grep -q 'karmada-workspace-apiserver.yaml$'; then
      cp "$file" "$KUBECTL_CAPTURE_DEPLOYMENT"
    fi
    exit 0
    ;;
  *)
    exit 0
    ;;
esac
`
	if err := os.WriteFile(filepath.Join(fakeBinDir, "kubectl"), []byte(kubectlScript), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fakeBinDir, "kind"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("bash", filepath.Join(repoDir, "deploy-workspace-apiserver.sh"), hostKubeconfig, "karmada-host", karmadaKubeconfig, "karmada-apiserver")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"PATH="+fakeBinDir+":"+os.Getenv("PATH"),
		"BUILD_FROM_SOURCE=false",
		"WAIT_FOR_READY=false",
		"REGISTRY=example.com/karmada",
		"VERSION=v0.0.1-test",
		"KUBECTL_LOG="+kubectlLog,
		"KUBECTL_CAPTURE_DEPLOYMENT="+renderedDeployment,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("deploy-workspace-apiserver.sh failed: %v\n%s", err, output)
	}

	deploymentData, err := os.ReadFile(renderedDeployment)
	if err != nil {
		t.Fatalf("expected rendered deployment manifest: %v", err)
	}
	if !strings.Contains(string(deploymentData), "example.com/karmada/karmada-workspace-apiserver:v0.0.1-test") {
		t.Fatalf("rendered deployment did not contain configured image: %s", deploymentData)
	}

	logData, err := os.ReadFile(kubectlLog)
	if err != nil {
		t.Fatal(err)
	}
	logText := string(logData)
	for _, want := range []string{
		"--context=karmada-host apply -f ",
		"karmada-workspace-apiserver-config-secret.yaml",
		"karmada-workspace-apiserver-cert-secret.yaml",
		"karmada-workspace-apiserver-etcd-client-cert-secret.yaml",
		"--context=karmada-apiserver apply -f ",
		"karmada-workspace-apiserver-apiservice.yaml",
	} {
		if !strings.Contains(logText, want) {
			t.Fatalf("expected kubectl log to contain %q, got:\n%s", want, logText)
		}
	}
}

func TestDeployWorkspaceApiserverFailsWhenContextIsMissing(t *testing.T) {
	repoDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	tempDir := t.TempDir()
	fakeBinDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(fakeBinDir, 0o755); err != nil {
		t.Fatal(err)
	}

	hostKubeconfig := filepath.Join(tempDir, "host.kubeconfig")
	karmadaKubeconfig := filepath.Join(tempDir, "karmada.kubeconfig")
	for _, path := range []string{hostKubeconfig, karmadaKubeconfig} {
		if err := os.WriteFile(path, []byte("apiVersion: v1\nkind: Config\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(fakeBinDir, "kubectl"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("bash", filepath.Join(repoDir, "deploy-workspace-apiserver.sh"), hostKubeconfig, "missing-context", karmadaKubeconfig, "karmada-apiserver")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "PATH="+fakeBinDir+":"+os.Getenv("PATH"))
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected deploy-workspace-apiserver.sh to fail, got output:\n%s", output)
	}
	if !strings.Contains(string(output), "failed to get context") {
		t.Fatalf("expected missing context error, got:\n%s", output)
	}
}

func TestUndeployWorkspaceApiserverDeletesWorkspaceResourcesAndCerts(t *testing.T) {
	repoDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	tempDir := t.TempDir()
	fakeBinDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(fakeBinDir, 0o755); err != nil {
		t.Fatal(err)
	}

	kubectlLog := filepath.Join(tempDir, "kubectl.log")
	kubectlScript := `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$KUBECTL_LOG"
if [ "${1:-}" = "config" ] && [ "${2:-}" = "get-contexts" ]; then
  case "${3:-}" in
    karmada-host|karmada-apiserver) exit 0 ;;
    *) exit 1 ;;
  esac
fi
exit 0
`
	if err := os.WriteFile(filepath.Join(fakeBinDir, "kubectl"), []byte(kubectlScript), 0o755); err != nil {
		t.Fatal(err)
	}

	homeDir := filepath.Join(tempDir, "home")
	certDir := filepath.Join(homeDir, ".karmada")
	if err := os.MkdirAll(certDir, 0o755); err != nil {
		t.Fatal(err)
	}
	certFiles := []string{
		"karmada-workspace-apiserver.crt",
		"karmada-workspace-apiserver.key",
		"karmada-workspace-apiserver-client.crt",
		"karmada-workspace-apiserver-client.key",
		"karmada-workspace-apiserver-etcd-client.crt",
		"karmada-workspace-apiserver-etcd-client.key",
	}
	for _, name := range certFiles {
		if err := os.WriteFile(filepath.Join(certDir, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	hostKubeconfig := filepath.Join(tempDir, "host.kubeconfig")
	karmadaKubeconfig := filepath.Join(tempDir, "karmada.kubeconfig")
	for _, path := range []string{hostKubeconfig, karmadaKubeconfig} {
		if err := os.WriteFile(path, []byte("apiVersion: v1\nkind: Config\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	cmd := exec.Command("bash", filepath.Join(repoDir, "undeploy-workspace-apiserver.sh"), hostKubeconfig, "karmada-host", karmadaKubeconfig, "karmada-apiserver")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"PATH="+fakeBinDir+":"+os.Getenv("PATH"),
		"KUBECTL_LOG="+kubectlLog,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("undeploy-workspace-apiserver.sh failed: %v\n%s", err, output)
	}

	for _, name := range certFiles {
		if _, err := os.Stat(filepath.Join(certDir, name)); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed, stat err=%v", name, err)
		}
	}

	logData, err := os.ReadFile(kubectlLog)
	if err != nil {
		t.Fatal(err)
	}
	logText := string(logData)
	for _, want := range []string{
		"--context=karmada-apiserver delete -f ",
		"karmada-workspace-apiserver-apiservice.yaml",
		"--context=karmada-host delete -f ",
		"karmada-workspace-apiserver.yaml",
		"delete secret karmada-workspace-apiserver-config",
		"delete secret karmada-workspace-apiserver-cert",
		"delete secret karmada-workspace-apiserver-etcd-client-cert",
	} {
		if !strings.Contains(logText, want) {
			t.Fatalf("expected kubectl log to contain %q, got:\n%s", want, logText)
		}
	}
}

func TestVerifyWorkspaceLocalUpRunsDeployAndWorkspaceChecks(t *testing.T) {
	repoDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	tempDir := t.TempDir()
	fakeBinDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(fakeBinDir, 0o755); err != nil {
		t.Fatal(err)
	}

	commandLog := filepath.Join(tempDir, "commands.log")
	fakeDeploy := filepath.Join(tempDir, "fake-deploy.sh")
	if err := os.WriteFile(fakeDeploy, []byte("#!/bin/sh\nprintf 'deploy:%s\\n' \"$*\" >> \"$COMMAND_LOG\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	fakeGo := filepath.Join(fakeBinDir, "go")
	if err := os.WriteFile(fakeGo, []byte("#!/bin/sh\nprintf 'go:%s\\n' \"$*\" >> \"$COMMAND_LOG\"\nif [ \"${1:-}\" = env ] && [ \"${2:-}\" = GOPATH ]; then\n  printf '%s\\n' \"${FAKE_GOPATH}\"\nfi\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	homeDir := filepath.Join(tempDir, "home")
	kubeDir := filepath.Join(homeDir, ".kube")
	if err := os.MkdirAll(kubeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(kubeDir, "karmada.config"), []byte("apiVersion: v1\nkind: Config\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("bash", filepath.Join(repoDir, "verify-workspace-local-up.sh"), "team-a")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"PATH="+fakeBinDir+":"+os.Getenv("PATH"),
		"COMMAND_LOG="+commandLog,
		"FAKE_GOPATH="+filepath.Join(tempDir, "gopath"),
		"RUN_GINKGO=false",
		"WORKSPACE_DEPLOY_SCRIPT="+fakeDeploy,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("verify-workspace-local-up.sh failed: %v\n%s", err, output)
	}

	logData, err := os.ReadFile(commandLog)
	if err != nil {
		t.Fatal(err)
	}
	logText := string(logData)
	if !strings.Contains(logText, "deploy:"+filepath.Join(homeDir, ".kube", "karmada.config")+" karmada-host "+filepath.Join(homeDir, ".kube", "karmada.config")+" karmada-apiserver") {
		t.Fatalf("expected deploy step in log, got:\n%s", logText)
	}
	if !strings.Contains(logText, "go:test ./test/e2e/suites/workspace -count=1") {
		t.Fatalf("expected workspace go test in log, got:\n%s", logText)
	}

	outputText := string(output)
	if !strings.Contains(outputText, "karmadactl workspace kubeconfig team-a") {
		t.Fatalf("expected manual kubeconfig command in output, got:\n%s", outputText)
	}
	if !strings.Contains(outputText, "k9s --kubeconfig /tmp/team-a.kubeconfig") {
		t.Fatalf("expected k9s handoff command in output, got:\n%s", outputText)
	}
}
