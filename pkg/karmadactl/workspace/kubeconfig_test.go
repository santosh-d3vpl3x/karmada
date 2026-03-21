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
	"testing"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestBuildWorkspaceKubeconfigUsesPublishedURL(t *testing.T) {
	baseConfig := newBaseKubeconfig()
	cfg, err := buildWorkspaceKubeconfig(baseConfig, "team-a", "https://karmada.example.com/apis/workspace.karmada.io/v1alpha1/workspaces/team-a/proxy/")
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Clusters["workspace-team-a"].Server; got != "https://karmada.example.com/apis/workspace.karmada.io/v1alpha1/workspaces/team-a/proxy/" {
		t.Fatalf("got %s", got)
	}
}

func TestBuildWorkspaceKubeconfigFallsBackToDeterministicURL(t *testing.T) {
	baseConfig := newBaseKubeconfig()
	cfg, err := buildWorkspaceKubeconfig(baseConfig, "team-a", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Clusters["workspace-team-a"].Server; got != "https://karmada.example.com/base/apis/workspace.karmada.io/v1alpha1/workspaces/team-a/proxy/" {
		t.Fatalf("got %s", got)
	}
}

func newBaseKubeconfig() *clientcmdapi.Config {
	return &clientcmdapi.Config{
		CurrentContext: "karmada",
		Clusters: map[string]*clientcmdapi.Cluster{
			"karmada": {Server: "https://karmada.example.com/base"},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"user": {},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"karmada": {Cluster: "karmada", AuthInfo: "user", Namespace: "team-a"},
		},
	}
}
