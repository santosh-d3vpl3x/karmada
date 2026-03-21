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
	"context"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	coretesting "k8s.io/client-go/testing"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	workspacev1alpha1 "github.com/karmada-io/karmada/pkg/apis/workspace/v1alpha1"
	fakeclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/fake"
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

func TestResolveWorkspaceURLUsesPublishedURL(t *testing.T) {
	client := fakeclientset.NewSimpleClientset(&workspacev1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{Name: "team-a"},
		Status: workspacev1alpha1.WorkspaceStatus{
			URL: "https://karmada.example.com/apis/workspace.karmada.io/v1alpha1/workspaces/team-a/proxy/",
		},
	})

	url, err := resolveWorkspaceURL(context.Background(), client.WorkspaceV1alpha1().Workspaces(), "team-a")
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://karmada.example.com/apis/workspace.karmada.io/v1alpha1/workspaces/team-a/proxy/" {
		t.Fatalf("got %s", url)
	}
}

func TestResolveWorkspaceURLFallsBackWhenGetIsMethodNotSupported(t *testing.T) {
	client := fakeclientset.NewSimpleClientset()
	client.PrependReactor("get", "workspaces", func(coretesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewMethodNotSupported(schema.GroupResource{Group: workspacev1alpha1.GroupName, Resource: workspacev1alpha1.ResourcePluralWorkspace}, "get")
	})

	url, err := resolveWorkspaceURL(context.Background(), client.WorkspaceV1alpha1().Workspaces(), "team-a")
	if err != nil {
		t.Fatal(err)
	}
	if url != "" {
		t.Fatalf("expected empty published URL, got %q", url)
	}
}

func TestResolveWorkspaceURLErrorOnOtherGetFailures(t *testing.T) {
	client := fakeclientset.NewSimpleClientset()
	client.PrependReactor("get", "workspaces", func(coretesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Group: workspacev1alpha1.GroupName, Resource: workspacev1alpha1.ResourcePluralWorkspace}, "team-a", nil)
	})

	if _, err := resolveWorkspaceURL(context.Background(), client.WorkspaceV1alpha1().Workspaces(), "team-a"); err == nil {
		t.Fatal("expected get error")
	}
}
