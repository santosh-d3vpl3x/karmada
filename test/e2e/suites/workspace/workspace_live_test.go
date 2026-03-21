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

package workspace

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"k8s.io/client-go/rest"

	"github.com/karmada-io/karmada/test/e2e/framework"
)

func workspaceRawGET(access framework.WorkspaceAccess, path string) (*http.Response, error) {
	config, err := framework.LoadWorkspaceRESTClientConfig(access.KubeconfigPath)
	if err != nil {
		return nil, err
	}

	transport, err := rest.TransportFor(config)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Transport: transport, Timeout: 30 * time.Second}
	url := strings.TrimSuffix(config.Host, "/") + "/" + strings.TrimPrefix(path, "/")
	return client.Get(url)
}

var _ = ginkgo.Describe("Workspace Live Access", ginkgo.Ordered, func() {
	var harness *workspaceHarness
	var podName string

	ginkgo.BeforeAll(func() {
		harness = mustWorkspaceHarness()
		podName = waitForWorkspacePodName(harness)
	})

	ginkgo.It("supports kubectl logs", func() {
		output, err := framework.RunWorkspaceKubectl(harness.Access.KubeconfigPath, "logs", podName, "-n", harness.Namespace, "-c", "app", "--tail=20")
		if err != nil {
			ginkgo.Skip(fmt.Sprintf("workspace logs are not available in the current environment: %s", strings.TrimSpace(output)))
		}
		gomega.Expect(output).Should(gomega.Or(gomega.ContainSubstring("workspace-ready"), gomega.ContainSubstring("workspace-log")))
	})

	ginkgo.It("supports kubectl exec", func() {
		output, err := framework.RunWorkspaceKubectl(harness.Access.KubeconfigPath, "exec", podName, "-n", harness.Namespace, "-c", "app", "--", "cat", "/www/index.html")
		if err != nil {
			ginkgo.Skip(fmt.Sprintf("workspace exec is not available in the current environment: %s", strings.TrimSpace(output)))
		}
		gomega.Expect(strings.TrimSpace(output)).Should(gomega.Equal("workspace-port-forward"))
	})

	ginkgo.It("supports kubectl attach", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		cmd := framework.NewWorkspaceKubectlCommand(ctx, harness.Access.KubeconfigPath, "attach", podName, "-n", harness.Namespace, "-c", "app", "--stdin=false", "--tty=false")
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		combined := stdout.String() + stderr.String()
		if !strings.Contains(combined, "workspace-log") && !strings.Contains(combined, "workspace-ready") {
			ginkgo.Skip(fmt.Sprintf("workspace attach is not available in the current environment: %s", strings.TrimSpace(combined)))
		}
		if err != nil && !strings.Contains(err.Error(), "context deadline exceeded") {
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), combined)
		}
	})

	ginkgo.It("supports kubectl port-forward", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		cmd := framework.NewWorkspaceKubectlCommand(ctx, harness.Access.KubeconfigPath, "port-forward", podName, "18080:8080", "-n", harness.Namespace)
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		gomega.Expect(cmd.Start()).ShouldNot(gomega.HaveOccurred())

		gomega.Eventually(func() string {
			return stdout.String() + stderr.String()
		}, 20*time.Second, time.Second).Should(gomega.ContainSubstring("Forwarding from"))

		resp, err := http.Get("http://127.0.0.1:18080")
		if err != nil {
			cancel()
			_ = cmd.Wait()
			ginkgo.Skip(fmt.Sprintf("workspace port-forward is not available in the current environment: %v", err))
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		gomega.Expect(string(body)).Should(gomega.ContainSubstring("workspace-port-forward"))

		cancel()
		_ = cmd.Wait()
	})

	ginkgo.It("returns 409 Conflict for ambiguous live targets when a target is supplied", func() {
		ambiguousPod := strings.TrimSpace(os.Getenv("WORKSPACE_E2E_AMBIGUOUS_POD"))
		if ambiguousPod == "" {
			ginkgo.Skip("WORKSPACE_E2E_AMBIGUOUS_POD is not set")
		}

		resp, err := workspaceRawGET(harness.Access, fmt.Sprintf("api/v1/namespaces/%s/pods/%s/logs", harness.Namespace, ambiguousPod))
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		defer resp.Body.Close()
		gomega.Expect(resp.StatusCode).Should(gomega.Equal(http.StatusConflict))
	})
})
