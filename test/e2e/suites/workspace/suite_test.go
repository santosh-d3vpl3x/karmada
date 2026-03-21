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
	"flag"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var (
	pollInterval    time.Duration
	pollTimeout     time.Duration
	karmadaContext  string
	kubeconfig      string
	karmadactlPath  string
	workspaceTmpDir string
)

func init() {
	flag.DurationVar(&pollInterval, "poll-interval", 5*time.Second, "poll-interval defines the interval time for workspace e2e polling")
	flag.DurationVar(&pollTimeout, "poll-timeout", 5*time.Minute, "poll-timeout defines the time after which workspace e2e polling times out")
	flag.StringVar(&karmadaContext, "karmada-context", "karmada-apiserver", "Name of the karmada cluster context in control plane kubeconfig file.")
}

func TestWorkspaceE2E(t *testing.T) {
	if os.Getenv("KUBECONFIG") == "" {
		t.Skip("KUBECONFIG is required for workspace e2e")
	}
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Workspace E2E Suite")
}

var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	return nil
}, func([]byte) {
	kubeconfig = os.Getenv("KUBECONFIG")
	gomega.Expect(kubeconfig).ShouldNot(gomega.BeEmpty())

	goPathCmd := exec.Command("go", "env", "GOPATH")
	goPath, err := goPathCmd.CombinedOutput()
	if err != nil {
		ginkgo.Skip("go env GOPATH is required to locate karmadactl")
	}

	formatGoPath := strings.TrimSpace(string(goPath))
	if formatGoPath == "" {
		ginkgo.Skip("GOPATH is empty; cannot locate karmadactl")
	}

	karmadactlPath = formatGoPath + "/bin/karmadactl"
	if _, err := os.Stat(karmadactlPath); err != nil {
		ginkgo.Skip("karmadactl binary is required for workspace e2e")
	}

	workspaceTmpDir, err = os.MkdirTemp("", "workspace-e2e-")
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
})

var _ = ginkgo.SynchronizedAfterSuite(func() {
	if workspaceTmpDir != "" {
		_ = os.RemoveAll(workspaceTmpDir)
	}
}, func() {})
