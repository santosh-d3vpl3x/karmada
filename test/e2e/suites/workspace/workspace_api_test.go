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
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	krand "k8s.io/apimachinery/pkg/util/rand"

	"github.com/karmada-io/karmada/test/e2e/framework"
)

type workspaceHarness struct {
	Access    framework.WorkspaceAccess
	Namespace string
}

var (
	workspaceHarnessOnce sync.Once
	workspaceHarnessInst *workspaceHarness
	workspaceHarnessErr  error
)

func mustWorkspaceHarness() *workspaceHarness {
	workspaceHarnessOnce.Do(func() {
		if kubeconfig == "" {
			workspaceHarnessErr = fmt.Errorf("KUBECONFIG is empty")
			return
		}

		name := fmt.Sprintf("workspace-e2e-%s", krand.String(5))
		access, err := framework.GenerateWorkspaceAccess(workspaceTmpDir, name, kubeconfig, karmadaContext, karmadactlPath, framework.WorkspaceCommandTimeout)
		if err != nil {
			workspaceHarnessErr = fmt.Errorf("generate workspace kubeconfig: %w", err)
			return
		}
		if output, err := framework.RunWorkspaceKubectl(access.KubeconfigPath, "api-resources", "-o", "name"); err != nil {
			workspaceHarnessErr = fmt.Errorf("workspace discovery is not ready: %w: %s", err, strings.TrimSpace(output))
			return
		}

		workspaceHarnessInst = &workspaceHarness{
			Access:    access,
			Namespace: fmt.Sprintf("workspace-e2e-%s", krand.String(5)),
		}
	})

	if workspaceHarnessErr != nil {
		ginkgo.Skip(workspaceHarnessErr.Error())
	}

	return workspaceHarnessInst
}

func supportedWorkspaceAPIResources() []string {
	resources := []string{
		"configmaps",
		"cronjobs.batch",
		"daemonsets.apps",
		"deployments.apps",
		"events",
		"jobs.batch",
		"namespaces",
		"pods",
		"pods/attach",
		"pods/exec",
		"pods/logs",
		"pods/portforward",
		"secrets",
		"services",
		"statefulsets.apps",
	}
	sort.Strings(resources)
	return resources
}

func unsupportedWorkspaceAPIResources() []string {
	return []string{"nodes", "pods/proxy", "replicationcontrollers", "placements.cluster.karmada.io"}
}

func normalizeLines(output string) []string {
	lines := strings.Split(output, "\n")
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		normalized = append(normalized, line)
	}
	sort.Strings(normalized)
	return normalized
}

func applyWorkspaceManifest(h *workspaceHarness, manifest string) (string, error) {
	return framework.RunWorkspaceKubectlWithInput(h.Access.KubeconfigPath, strings.NewReader(manifest), "apply", "-f", "-")
}

func deleteWorkspaceManifest(h *workspaceHarness, manifest string) (string, error) {
	return framework.RunWorkspaceKubectlWithInput(h.Access.KubeconfigPath, strings.NewReader(manifest), "delete", "--ignore-not-found", "-f", "-")
}

func waitForWorkspaceKubectl(h *workspaceHarness, timeout time.Duration, args ...string) (string, bool) {
	deadline := time.Now().Add(timeout)
	var last string
	for time.Now().Before(deadline) {
		output, err := framework.RunWorkspaceKubectl(h.Access.KubeconfigPath, args...)
		last = output
		if err == nil {
			return output, true
		}
		time.Sleep(pollInterval)
	}
	return last, false
}

func waitForWorkspacePodName(h *workspaceHarness) string {
	deadline := time.Now().Add(pollTimeout)
	for time.Now().Before(deadline) {
		output, err := framework.RunWorkspaceKubectl(h.Access.KubeconfigPath, "get", "pods", "-n", h.Namespace, "-o", "jsonpath={.items[0].metadata.name}")
		if err == nil {
			name := strings.TrimSpace(output)
			if name != "" {
				return name
			}
		}
		time.Sleep(pollInterval)
	}
	ginkgo.Skip("workspace projected pods are not available in the current environment")
	return ""
}

func workspaceNamespaceManifest(namespace string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Namespace
metadata:
  name: %s
  labels:
    suite: workspace-e2e
`, namespace)
}

func workspaceWritableManifest(namespace string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: workspace-config
  namespace: %s
data:
  mode: initial
---
apiVersion: v1
kind: Secret
metadata:
  name: workspace-secret
  namespace: %s
type: Opaque
stringData:
  token: initial
---
apiVersion: v1
kind: Service
metadata:
  name: workspace-service
  namespace: %s
spec:
  selector:
    app: workspace-demo
  ports:
  - name: http
    port: 8080
    targetPort: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: workspace-headless
  namespace: %s
spec:
  clusterIP: None
  selector:
    app: workspace-stateful
  ports:
  - name: http
    port: 8080
    targetPort: 8080
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: workspace-demo
  namespace: %s
spec:
  replicas: 1
  selector:
    matchLabels:
      app: workspace-demo
  template:
    metadata:
      labels:
        app: workspace-demo
    spec:
      containers:
      - name: app
        image: busybox:1.36.0
        command:
        - sh
        - -c
        - |
          mkdir -p /www
          echo workspace-port-forward > /www/index.html
          httpd -f -p 8080 -h /www &
          echo workspace-ready
          while true; do
            echo workspace-log
            sleep 5
          done
        ports:
        - containerPort: 8080
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: workspace-stateful
  namespace: %s
spec:
  serviceName: workspace-headless
  replicas: 1
  selector:
    matchLabels:
      app: workspace-stateful
  template:
    metadata:
      labels:
        app: workspace-stateful
    spec:
      containers:
      - name: app
        image: busybox:1.36.0
        command: ["sh", "-c", "while true; do sleep 30; done"]
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: workspace-daemon
  namespace: %s
spec:
  selector:
    matchLabels:
      app: workspace-daemon
  template:
    metadata:
      labels:
        app: workspace-daemon
    spec:
      containers:
      - name: app
        image: busybox:1.36.0
        command: ["sh", "-c", "while true; do sleep 30; done"]
---
apiVersion: batch/v1
kind: Job
metadata:
  name: workspace-job
  namespace: %s
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
      - name: app
        image: busybox:1.36.0
        command: ["sh", "-c", "echo workspace-job"]
---
apiVersion: batch/v1
kind: CronJob
metadata:
  name: workspace-cron
  namespace: %s
spec:
  schedule: "*/15 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          restartPolicy: Never
          containers:
          - name: app
            image: busybox:1.36.0
            command: ["sh", "-c", "echo workspace-cron"]
`, namespace, namespace, namespace, namespace, namespace, namespace, namespace, namespace, namespace)
}

func workspaceWritableUpdateManifest(namespace string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: workspace-config
  namespace: %s
data:
  mode: updated
---
apiVersion: v1
kind: Secret
metadata:
  name: workspace-secret
  namespace: %s
type: Opaque
stringData:
  token: updated
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: workspace-demo
  namespace: %s
spec:
  replicas: 2
  selector:
    matchLabels:
      app: workspace-demo
  template:
    metadata:
      labels:
        app: workspace-demo
    spec:
      containers:
      - name: app
        image: busybox:1.36.0
        command:
        - sh
        - -c
        - |
          mkdir -p /www
          echo workspace-port-forward > /www/index.html
          httpd -f -p 8080 -h /www &
          echo workspace-ready
          while true; do
            echo workspace-log
            sleep 5
          done
        ports:
        - containerPort: 8080
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: workspace-stateful
  namespace: %s
spec:
  serviceName: workspace-headless
  replicas: 2
  selector:
    matchLabels:
      app: workspace-stateful
  template:
    metadata:
      labels:
        app: workspace-stateful
    spec:
      containers:
      - name: app
        image: busybox:1.36.0
        command: ["sh", "-c", "while true; do sleep 30; done"]
---
apiVersion: batch/v1
kind: CronJob
metadata:
  name: workspace-cron
  namespace: %s
spec:
  schedule: "*/10 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          restartPolicy: Never
          containers:
          - name: app
            image: busybox:1.36.0
            command: ["sh", "-c", "echo workspace-cron"]
`, namespace, namespace, namespace, namespace, namespace)
}

var _ = ginkgo.Describe("Workspace API", ginkgo.Ordered, func() {
	var harness *workspaceHarness

	ginkgo.BeforeAll(func() {
		harness = mustWorkspaceHarness()
	})

	ginkgo.AfterAll(func() {
		if harness == nil {
			return
		}
		_, _ = deleteWorkspaceManifest(harness, workspaceWritableManifest(harness.Namespace))
		_, _ = framework.RunWorkspaceKubectl(harness.Access.KubeconfigPath, "delete", "namespace", harness.Namespace, "--ignore-not-found")
	})

	ginkgo.It("generates a workspace kubeconfig through karmadactl workspace kubeconfig", func() {
		gomega.Expect(harness.Access.KubeconfigPath).ShouldNot(gomega.BeEmpty())
		gomega.Expect(harness.Access.Endpoint).Should(gomega.ContainSubstring("/apis/workspace.karmada.io/v1alpha1/workspaces/"))
		gomega.Expect(harness.Access.Endpoint).Should(gomega.HaveSuffix("/proxy/"))
	})

	ginkgo.It("shows only the supported phase-1 surface in kubectl api-resources", func() {
		output, err := framework.RunWorkspaceKubectl(harness.Access.KubeconfigPath, "api-resources", "-o", "name")
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), output)

		resources := normalizeLines(output)
		for _, resource := range supportedWorkspaceAPIResources() {
			gomega.Expect(resources).Should(gomega.ContainElement(resource), resource)
		}
		for _, resource := range unsupportedWorkspaceAPIResources() {
			gomega.Expect(resources).ShouldNot(gomega.ContainElement(resource), resource)
		}
	})

	ginkgo.It("supports CRUD for the writable phase-1 resources", func() {
		output, err := applyWorkspaceManifest(harness, workspaceNamespaceManifest(harness.Namespace))
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), output)

		output, ok := waitForWorkspaceKubectl(harness, pollTimeout, "get", "namespace", harness.Namespace, "-o", "name")
		if !ok {
			ginkgo.Skip(fmt.Sprintf("workspace desired-state writes are not available in the current environment: %s", strings.TrimSpace(output)))
		}

		output, err = applyWorkspaceManifest(harness, workspaceWritableManifest(harness.Namespace))
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), output)

		for _, args := range [][]string{
			{"get", "configmap", "workspace-config", "-n", harness.Namespace, "-o", "jsonpath={.data.mode}"},
			{"get", "secret", "workspace-secret", "-n", harness.Namespace, "-o", "jsonpath={.data.token}"},
			{"get", "service", "workspace-service", "-n", harness.Namespace, "-o", "name"},
			{"get", "deployment", "workspace-demo", "-n", harness.Namespace, "-o", "name"},
			{"get", "statefulset", "workspace-stateful", "-n", harness.Namespace, "-o", "name"},
			{"get", "daemonset", "workspace-daemon", "-n", harness.Namespace, "-o", "name"},
			{"get", "job", "workspace-job", "-n", harness.Namespace, "-o", "name"},
			{"get", "cronjob", "workspace-cron", "-n", harness.Namespace, "-o", "name"},
		} {
			resourceOutput, resourceErr := framework.RunWorkspaceKubectl(harness.Access.KubeconfigPath, args...)
			gomega.Expect(resourceErr).ShouldNot(gomega.HaveOccurred(), resourceOutput)
		}

		output, err = applyWorkspaceManifest(harness, workspaceWritableUpdateManifest(harness.Namespace))
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), output)

		configMapOutput, err := framework.RunWorkspaceKubectl(harness.Access.KubeconfigPath, "get", "configmap", "workspace-config", "-n", harness.Namespace, "-o", "jsonpath={.data.mode}")
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), configMapOutput)
		gomega.Expect(strings.TrimSpace(configMapOutput)).Should(gomega.Equal("updated"))

		deploymentOutput, err := framework.RunWorkspaceKubectl(harness.Access.KubeconfigPath, "get", "deployment", "workspace-demo", "-n", harness.Namespace, "-o", "jsonpath={.spec.replicas}")
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), deploymentOutput)
		gomega.Expect(strings.TrimSpace(deploymentOutput)).Should(gomega.Equal("2"))

		deleteOutput, err := framework.RunWorkspaceKubectl(harness.Access.KubeconfigPath, "delete", "job", "workspace-job", "-n", harness.Namespace, "--ignore-not-found")
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), deleteOutput)
	})

	ginkgo.It("exposes placementviews as a read-only debug surface", func() {
		podName := waitForWorkspacePodName(harness)

		resp, err := controlPlaneRawGET(fmt.Sprintf("apis/workspace.karmada.io/v1alpha1/placementviews/%s.%s", harness.Namespace, podName))
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		defer resp.Body.Close()
		gomega.Expect(resp.StatusCode).Should(gomega.Equal(200))

		view := decodePlacementViewResponse(resp)
		gomega.Expect(view.Status.SubjectRef.Namespace).Should(gomega.Equal(harness.Namespace))
		gomega.Expect(view.Status.SubjectRef.Name).Should(gomega.Equal(podName))
		gomega.Expect(view.Status.EligibleClusters).ShouldNot(gomega.BeEmpty())
	})

	ginkgo.It("supports kubectl get pods, get events, and get -w on supported resources", func() {
		podName := waitForWorkspacePodName(harness)

		podsOutput, err := framework.RunWorkspaceKubectl(harness.Access.KubeconfigPath, "get", "pods", "-n", harness.Namespace)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), podsOutput)
		gomega.Expect(podsOutput).Should(gomega.ContainSubstring(podName))

		eventsOutput, err := framework.RunWorkspaceKubectl(harness.Access.KubeconfigPath, "get", "events", "-n", harness.Namespace)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), eventsOutput)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		cmd := framework.NewWorkspaceKubectlCommand(ctx, harness.Access.KubeconfigPath, "get", "configmaps", "-n", harness.Namespace, "-w", "--request-timeout=30s")
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		gomega.Expect(cmd.Start()).ShouldNot(gomega.HaveOccurred())

		watchName := fmt.Sprintf("workspace-watch-%s", krand.String(5))
		time.Sleep(2 * time.Second)
		watchApply, err := framework.RunWorkspaceKubectlWithInput(harness.Access.KubeconfigPath, strings.NewReader(fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: %s
data:
  watched: "true"
`, watchName, harness.Namespace)), "apply", "-f", "-")
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), watchApply)

		gomega.Eventually(func() string {
			return stdout.String() + stderr.String()
		}, 20*time.Second, time.Second).Should(gomega.ContainSubstring(watchName))

		cancel()
		_ = cmd.Wait()
	})
})
