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
	discoveryfake "k8s.io/client-go/discovery/fake"
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
	client := newWorkspaceClientWithDiscovery(&workspacev1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{Name: "team-a"},
		Status: workspacev1alpha1.WorkspaceStatus{
			URL: "https://karmada.example.com/apis/workspace.karmada.io/v1alpha1/workspaces/team-a/proxy/",
		},
	}, []string{"get", "list"})

	url, err := resolveWorkspaceURL(context.Background(), client.Discovery(), client.WorkspaceV1alpha1().Workspaces(), "team-a")
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://karmada.example.com/apis/workspace.karmada.io/v1alpha1/workspaces/team-a/proxy/" {
		t.Fatalf("got %s", url)
	}
}

func TestResolveWorkspaceURLFallsBackWhenDiscoveryDoesNotAdvertiseGet(t *testing.T) {
	client := newWorkspaceClientWithDiscovery(nil, []string{"create", "update"})

	url, err := resolveWorkspaceURL(context.Background(), client.Discovery(), client.WorkspaceV1alpha1().Workspaces(), "team-a")
	if err != nil {
		t.Fatal(err)
	}
	if url != "" {
		t.Fatalf("expected empty published URL, got %q", url)
	}
	for _, action := range client.Actions() {
		if action.GetVerb() == "get" && action.GetResource().Resource == workspacev1alpha1.ResourcePluralWorkspace {
			t.Fatalf("expected no workspace get action, got %#v", action)
		}
	}
}

func TestResolveWorkspaceURLErrorOnOtherGetFailures(t *testing.T) {
	client := newWorkspaceClientWithDiscovery(nil, []string{"get", "list"})
	client.PrependReactor("get", "workspaces", func(coretesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Group: workspacev1alpha1.GroupName, Resource: workspacev1alpha1.ResourcePluralWorkspace}, "team-a", nil)
	})

	if _, err := resolveWorkspaceURL(context.Background(), client.Discovery(), client.WorkspaceV1alpha1().Workspaces(), "team-a"); err == nil {
		t.Fatal("expected get error")
	}
}

func newWorkspaceClientWithDiscovery(workspace runtime.Object, verbs []string) *fakeclientset.Clientset {
	var objects []runtime.Object
	if workspace != nil {
		objects = append(objects, workspace)
	}
	client := fakeclientset.NewSimpleClientset(objects...)
	discovery := client.Discovery().(*discoveryfake.FakeDiscovery)
	discovery.Resources = []*metav1.APIResourceList{{
		GroupVersion: workspacev1alpha1.SchemeGroupVersion.String(),
		APIResources: []metav1.APIResource{{
			Name:         workspacev1alpha1.ResourcePluralWorkspace,
			SingularName: workspacev1alpha1.ResourceSingularWorkspace,
			Namespaced:   false,
			Kind:         workspacev1alpha1.ResourceKindWorkspace,
			Verbs:        metav1.Verbs(verbs),
		}},
	}}
	return client
}
