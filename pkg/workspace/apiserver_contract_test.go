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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	genericapifilters "k8s.io/apiserver/pkg/endpoints/filters"
	"k8s.io/apiserver/pkg/endpoints/openapi"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/util/compatibility"
	"k8s.io/client-go/informers"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	restclient "k8s.io/client-go/rest"

	workspacescheme "github.com/karmada-io/karmada/pkg/apis/workspace/scheme"
	workspacev1alpha1 "github.com/karmada-io/karmada/pkg/apis/workspace/v1alpha1"
	generatedopenapi "github.com/karmada-io/karmada/pkg/generated/openapi"
	"github.com/karmada-io/karmada/pkg/workspace/facade"
	"github.com/karmada-io/karmada/pkg/workspace/index"
	"github.com/karmada-io/karmada/pkg/workspace/live"
	framework "github.com/karmada-io/karmada/test/e2e/framework"
)

func TestWorkspaceAPIServerRootDiscoveryAdvertisesWorkspaceResources(t *testing.T) {
	server := newTestWorkspaceServer(t, testWorkspaceServerOptions{})

	resources := discoverWorkspaceResources(t, server)
	if !containsResource(resources, workspacev1alpha1.ResourcePluralWorkspace) {
		t.Fatalf("expected %s in discovery", workspacev1alpha1.ResourcePluralWorkspace)
	}
	if !containsResource(resources, workspacev1alpha1.ResourcePluralPlacementView) {
		t.Fatalf("expected %s in discovery", workspacev1alpha1.ResourcePluralPlacementView)
	}
	if !containsResource(resources, workspacev1alpha1.ResourcePluralWorkspace+"/proxy") {
		t.Fatalf("expected %s/proxy in discovery", workspacev1alpha1.ResourcePluralWorkspace)
	}
}

func TestWorkspaceOfflineKubectlDiscoverySmoke(t *testing.T) {
	kubectlPath, err := framework.LookupWorkspaceKubectl()
	if err != nil {
		t.Skipf("workspace kubectl smoke requires kubectl: %v", err)
	}
	t.Setenv(framework.WorkspaceKubectlBinEnv, kubectlPath)

	server := newTestWorkspaceServer(t, testWorkspaceServerOptions{runtimeState: defaultRuntimeState()})
	kubeconfigPath := filepath.Join(t.TempDir(), "workspace-offline.kubeconfig")
	kubeconfig := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- name: workspace
  cluster:
    server: %s
contexts:
- name: workspace
  context:
    cluster: workspace
    user: workspace
current-context: workspace
users:
- name: workspace
  user: {}
`, server.URL)
	if err := os.WriteFile(kubeconfigPath, []byte(kubeconfig), 0o600); err != nil {
		t.Fatal(err)
	}

	output, err := framework.RunWorkspaceKubectl(kubeconfigPath, "api-resources", "-o", "name")
	if err != nil {
		t.Fatalf("kubectl api-resources failed: %v\n%s", err, output)
	}
	for _, want := range []string{"namespaces", "pods", "deployments.apps", "jobs.batch", "ingresses.networking.k8s.io", "rolebindings.rbac.authorization.k8s.io"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected kubectl discovery output to contain %q, got:\n%s", want, output)
		}
	}
	for _, unwanted := range []string{"nodes", "clusterroles.rbac.authorization.k8s.io", "customresourcedefinitions.apiextensions.k8s.io"} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("did not expect kubectl discovery output to contain %q, got:\n%s", unwanted, output)
		}
	}
}

func TestWorkspaceDiscoveryOnlyAdvertisesSupportedResources(t *testing.T) {
	server := newTestWorkspaceServer(t, testWorkspaceServerOptions{runtimeState: defaultRuntimeState()})

	groups := discoverAPIGroups(t, server, workspaceProxyPath("team-a", "/apis"))
	for _, name := range []string{"apps", "autoscaling", "batch", "networking.k8s.io", "policy", "rbac.authorization.k8s.io"} {
		if !containsGroup(groups, name) {
			t.Fatalf("expected %s in workspace api group discovery", name)
		}
	}
	for _, name := range []string{"apiextensions.k8s.io", "example.io"} {
		if containsGroup(groups, name) {
			t.Fatalf("did not expect %s in workspace api group discovery", name)
		}
	}

	coreResources := discoverAPIResources(t, server, workspaceProxyPath("team-a", "/api/v1"))
	if containsResource(coreResources, "nodes") {
		t.Fatal("did not expect nodes in workspace discovery")
	}
	for _, name := range []string{"namespaces", "configmaps", "secrets", "serviceaccounts", "limitranges", "resourcequotas", "services", "pods", "events"} {
		if !containsResource(coreResources, name) {
			t.Fatalf("expected %s in workspace core discovery", name)
		}
	}
	if containsResource(coreResources, "pods/proxy") {
		t.Fatal("did not expect pods/proxy in workspace discovery")
	}

	appsResources := discoverAPIResources(t, server, workspaceProxyPath("team-a", "/apis/apps/v1"))
	for _, name := range []string{"deployments", "statefulsets", "daemonsets"} {
		if !containsResource(appsResources, name) {
			t.Fatalf("expected %s in workspace apps discovery", name)
		}
	}

	batchResources := discoverAPIResources(t, server, workspaceProxyPath("team-a", "/apis/batch/v1"))
	for _, name := range []string{"jobs", "cronjobs"} {
		if !containsResource(batchResources, name) {
			t.Fatalf("expected %s in workspace batch discovery", name)
		}
	}

	networkingResources := discoverAPIResources(t, server, workspaceProxyPath("team-a", "/apis/networking.k8s.io/v1"))
	for _, name := range []string{"networkpolicies", "ingresses"} {
		if !containsResource(networkingResources, name) {
			t.Fatalf("expected %s in workspace networking discovery", name)
		}
	}
	networkPolicyResource, ok := apiResourceByName(networkingResources, "networkpolicies")
	if !ok {
		t.Fatal("expected networkpolicies api resource")
	}
	if networkPolicyResource.SingularName != "networkpolicy" {
		t.Fatalf("got networkpolicy singular name %q", networkPolicyResource.SingularName)
	}
	ingressResource, ok := apiResourceByName(networkingResources, "ingresses")
	if !ok {
		t.Fatal("expected ingresses api resource")
	}
	if ingressResource.SingularName != "ingress" {
		t.Fatalf("got ingress singular name %q", ingressResource.SingularName)
	}

	rbacResources := discoverAPIResources(t, server, workspaceProxyPath("team-a", "/apis/rbac.authorization.k8s.io/v1"))
	for _, name := range []string{"roles", "rolebindings"} {
		if !containsResource(rbacResources, name) {
			t.Fatalf("expected %s in workspace rbac discovery", name)
		}
	}
	if containsResource(rbacResources, "clusterroles") {
		t.Fatal("did not expect clusterroles in workspace rbac discovery")
	}

	policyResources := discoverAPIResources(t, server, workspaceProxyPath("team-a", "/apis/policy/v1"))
	if !containsResource(policyResources, "poddisruptionbudgets") {
		t.Fatal("expected poddisruptionbudgets in workspace policy discovery")
	}

	autoscalingResources := discoverAPIResources(t, server, workspaceProxyPath("team-a", "/apis/autoscaling/v2"))
	if !containsResource(autoscalingResources, "horizontalpodautoscalers") {
		t.Fatal("expected horizontalpodautoscalers in workspace autoscaling discovery")
	}
}

func TestWorkspaceServiceAccountWriteContract(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("got method %q", req.Method)
		}
		if req.URL.Path != "/api/v1/namespaces/default/serviceaccounts" {
			t.Fatalf("got path %q", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		_ = req.Body.Close()

		forwarded := &corev1.ServiceAccount{}
		if err := json.Unmarshal(body, forwarded); err != nil {
			t.Fatal(err)
		}
		if forwarded.Annotations[facade.AnnotationWorkspaceName] != "team-a" {
			t.Fatalf("missing workspace annotation: %#v", forwarded.Annotations)
		}
		if forwarded.Labels[facade.LabelWorkspaceName] != "team-a" {
			t.Fatalf("missing workspace label: %#v", forwarded.Labels)
		}

		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(rw).Encode(forwarded)
	}))
	defer backend.Close()

	backendURL, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatal(err)
	}
	server := newTestWorkspaceServer(t, testWorkspaceServerOptions{
		runtimeState:     defaultRuntimeState(),
		backendURL:       backendURL,
		backendTransport: backend.Client().Transport,
	})

	serviceAccount := corev1.ServiceAccount{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ServiceAccount"},
		ObjectMeta: metav1.ObjectMeta{Name: "builder", Namespace: "default"},
	}
	body, err := json.Marshal(&serviceAccount)
	if err != nil {
		t.Fatal(err)
	}

	resp := doJSONRequest(t, server, http.MethodPost, workspaceProxyPath("team-a", "/api/v1/namespaces/default/serviceaccounts"), body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("got %d", resp.StatusCode)
	}
}

func TestWorkspaceNetworkPolicyWriteContract(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("got method %q", req.Method)
		}
		if req.URL.Path != "/apis/networking.k8s.io/v1/namespaces/default/networkpolicies" {
			t.Fatalf("got path %q", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		_ = req.Body.Close()

		forwarded := &networkingv1.NetworkPolicy{}
		if err := json.Unmarshal(body, forwarded); err != nil {
			t.Fatal(err)
		}
		if forwarded.Annotations[facade.AnnotationWorkspaceName] != "team-a" {
			t.Fatalf("missing workspace annotation: %#v", forwarded.Annotations)
		}
		if forwarded.Labels[facade.LabelWorkspaceName] != "team-a" {
			t.Fatalf("missing workspace label: %#v", forwarded.Labels)
		}

		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(rw).Encode(forwarded)
	}))
	defer backend.Close()

	backendURL, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatal(err)
	}
	server := newTestWorkspaceServer(t, testWorkspaceServerOptions{
		runtimeState:     defaultRuntimeState(),
		backendURL:       backendURL,
		backendTransport: backend.Client().Transport,
	})

	networkPolicy := networkingv1.NetworkPolicy{
		TypeMeta:   metav1.TypeMeta{APIVersion: "networking.k8s.io/v1", Kind: "NetworkPolicy"},
		ObjectMeta: metav1.ObjectMeta{Name: "default-deny", Namespace: "default"},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
		},
	}
	body, err := json.Marshal(&networkPolicy)
	if err != nil {
		t.Fatal(err)
	}

	resp := doJSONRequest(t, server, http.MethodPost, workspaceProxyPath("team-a", "/apis/networking.k8s.io/v1/namespaces/default/networkpolicies"), body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("got %d", resp.StatusCode)
	}
}

func TestWorkspaceRoleWriteContract(t *testing.T) {
	assertWorkspaceWriteContract(t,
		workspaceProxyPath("team-a", "/apis/rbac.authorization.k8s.io/v1/namespaces/default/roles"),
		"/apis/rbac.authorization.k8s.io/v1/namespaces/default/roles",
		map[string]any{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       "Role",
			"metadata": map[string]any{
				"name":      "workspace-reader",
				"namespace": "default",
			},
			"rules": []any{map[string]any{
				"apiGroups": []any{""},
				"resources": []any{"configmaps"},
				"verbs":     []any{"get", "list"},
			}},
		},
	)
}

func TestWorkspaceIngressWriteContract(t *testing.T) {
	assertWorkspaceWriteContract(t,
		workspaceProxyPath("team-a", "/apis/networking.k8s.io/v1/namespaces/default/ingresses"),
		"/apis/networking.k8s.io/v1/namespaces/default/ingresses",
		map[string]any{
			"apiVersion": "networking.k8s.io/v1",
			"kind":       "Ingress",
			"metadata": map[string]any{
				"name":      "workspace-http",
				"namespace": "default",
			},
			"spec": map[string]any{
				"rules": []any{map[string]any{
					"http": map[string]any{
						"paths": []any{map[string]any{
							"path":     "/",
							"pathType": "Prefix",
							"backend": map[string]any{
								"service": map[string]any{
									"name": "workspace-service",
									"port": map[string]any{"number": 8080},
								},
							},
						}},
					},
				}},
			},
		},
	)
}

func TestWorkspacePodDisruptionBudgetWriteContract(t *testing.T) {
	assertWorkspaceWriteContract(t,
		workspaceProxyPath("team-a", "/apis/policy/v1/namespaces/default/poddisruptionbudgets"),
		"/apis/policy/v1/namespaces/default/poddisruptionbudgets",
		map[string]any{
			"apiVersion": "policy/v1",
			"kind":       "PodDisruptionBudget",
			"metadata": map[string]any{
				"name":      "workspace-demo",
				"namespace": "default",
			},
			"spec": map[string]any{
				"minAvailable": 1,
				"selector":     map[string]any{"matchLabels": map[string]any{"app": "workspace-demo"}},
			},
		},
	)
}

func TestWorkspaceHorizontalPodAutoscalerWriteContract(t *testing.T) {
	assertWorkspaceWriteContract(t,
		workspaceProxyPath("team-a", "/apis/autoscaling/v2/namespaces/default/horizontalpodautoscalers"),
		"/apis/autoscaling/v2/namespaces/default/horizontalpodautoscalers",
		map[string]any{
			"apiVersion": "autoscaling/v2",
			"kind":       "HorizontalPodAutoscaler",
			"metadata": map[string]any{
				"name":      "workspace-demo",
				"namespace": "default",
			},
			"spec": map[string]any{
				"scaleTargetRef": map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"name":       "workspace-demo",
				},
				"minReplicas": 1,
				"maxReplicas": 3,
				"metrics": []any{map[string]any{
					"type": "Resource",
					"resource": map[string]any{
						"name": "cpu",
						"target": map[string]any{
							"type":               "Utilization",
							"averageUtilization": 80,
						},
					},
				}},
			},
		},
	)
}

func TestWorkspaceNamespaceDiscoveryAdvertisesWritableLogicalView(t *testing.T) {
	server := newTestWorkspaceServer(t, testWorkspaceServerOptions{runtimeState: defaultRuntimeState()})

	coreResources := discoverAPIResources(t, server, workspaceProxyPath("team-a", "/api/v1"))
	resource, ok := apiResourceByName(coreResources, "namespaces")
	if !ok {
		t.Fatal("expected namespaces in workspace core discovery")
	}
	if resource.Namespaced {
		t.Fatal("namespace view must remain cluster-scoped")
	}
	for _, verb := range []string{"get", "list", "watch", "create", "update", "patch", "delete", "deletecollection"} {
		if !containsString(resource.Verbs, verb) {
			t.Fatalf("expected namespace discovery verb %q", verb)
		}
	}
}

func TestWorkspaceWriteCompilesDesiredStateMetadata(t *testing.T) {
	backend := newRecordingBackend(t)
	server := newTestWorkspaceServer(t, testWorkspaceServerOptions{backend: backend})

	deployment := appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx",
			Namespace: "default",
		},
	}
	body, err := json.Marshal(&deployment)
	if err != nil {
		t.Fatal(err)
	}

	resp := doJSONRequest(t, server, http.MethodPost, workspaceProxyPath("team-a", "/apis/apps/v1/namespaces/default/deployments"), body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("got %d", resp.StatusCode)
	}

	forwarded := backend.decodeLastObject(t)
	annotations := forwarded.GetAnnotations()
	if annotations[facade.AnnotationWorkspaceName] != "team-a" {
		t.Fatalf("missing workspace annotation: %#v", annotations)
	}
	if annotations[facade.AnnotationWorkspaceResource] == "" {
		t.Fatal("missing workspace resource annotation")
	}
	if forwarded.GetLabels()[facade.LabelWorkspaceName] != "team-a" {
		t.Fatalf("missing workspace label: %#v", forwarded.GetLabels())
	}
}

func TestWorkspaceUnsupportedResourceContract(t *testing.T) {
	server := newTestWorkspaceServer(t, testWorkspaceServerOptions{runtimeState: defaultRuntimeState()})

	assertStatusCode(t, server, workspaceProxyPath("team-a", "/api/v1/nodes"), http.StatusNotFound)
	assertStatusCode(t, server, workspaceProxyPath("team-a", "/api/v1/namespaces/default/pods/demo/proxy"), http.StatusNotFound)
}

func TestWorkspaceUnsupportedVerbContract(t *testing.T) {
	server := newTestWorkspaceServer(t, testWorkspaceServerOptions{runtimeState: defaultRuntimeState()})

	resp := doJSONRequest(t, server, http.MethodPost, workspaceProxyPath("team-a", "/api/v1/namespaces/default/pods"), []byte(`{"apiVersion":"v1","kind":"Pod"}`))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("got %d", resp.StatusCode)
	}
}

func TestWorkspaceNamespaceWriteContract(t *testing.T) {
	backend := newRecordingBackend(t)
	server := newTestWorkspaceServer(t, testWorkspaceServerOptions{backend: backend, runtimeState: defaultRuntimeState()})

	namespace := corev1.Namespace{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
	}
	body, err := json.Marshal(&namespace)
	if err != nil {
		t.Fatal(err)
	}

	resp := doJSONRequest(t, server, http.MethodPost, workspaceProxyPath("team-a", "/api/v1/namespaces"), body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("got %d", resp.StatusCode)
	}
	if backend.lastRequest == nil {
		t.Fatal("expected desired-state request to be forwarded")
	}
	if backend.lastRequest.URL.Path != "/api/v1/namespaces" {
		t.Fatalf("got path %q", backend.lastRequest.URL.Path)
	}

	forwarded := &corev1.Namespace{}
	if err := json.Unmarshal(backend.lastBody, forwarded); err != nil {
		t.Fatal(err)
	}
	annotations := forwarded.GetAnnotations()
	if annotations[facade.AnnotationWorkspaceName] != "team-a" {
		t.Fatalf("missing workspace annotation: %#v", annotations)
	}
	if annotations[facade.AnnotationWorkspaceResource] == "" {
		t.Fatal("missing workspace resource annotation")
	}
	if forwarded.GetLabels()[facade.LabelWorkspaceName] != "team-a" {
		t.Fatalf("missing workspace label: %#v", forwarded.GetLabels())
	}
}

func TestWorkspaceNamespaceUpdateContract(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPut {
			t.Fatalf("got method %q", req.Method)
		}
		if req.URL.Path != "/api/v1/namespaces/demo" {
			t.Fatalf("got path %q", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		_ = req.Body.Close()

		forwarded := &corev1.Namespace{}
		if err := json.Unmarshal(body, forwarded); err != nil {
			t.Fatal(err)
		}
		if forwarded.Annotations[facade.AnnotationWorkspaceName] != "team-a" {
			t.Fatalf("missing workspace annotation: %#v", forwarded.Annotations)
		}
		if forwarded.Labels[facade.LabelWorkspaceName] != "team-a" {
			t.Fatalf("missing workspace label: %#v", forwarded.Labels)
		}

		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(forwarded)
	}))
	defer backend.Close()

	backendURL, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatal(err)
	}
	server := newTestWorkspaceServer(t, testWorkspaceServerOptions{
		runtimeState:     defaultRuntimeState(),
		backendURL:       backendURL,
		backendTransport: backend.Client().Transport,
	})

	namespace := corev1.Namespace{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
	}
	body, err := json.Marshal(&namespace)
	if err != nil {
		t.Fatal(err)
	}

	resp := doJSONRequest(t, server, http.MethodPut, workspaceProxyPath("team-a", "/api/v1/namespaces/demo"), body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got %d", resp.StatusCode)
	}
}

func TestWorkspaceNamespaceDeleteContract(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("got method %q", req.Method)
		}
		if req.URL.Path != "/api/v1/namespaces/demo" {
			t.Fatalf("got path %q", req.URL.Path)
		}
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(&metav1.Status{
			TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Status"},
			Status:   metav1.StatusSuccess,
			Code:     http.StatusOK,
		})
	}))
	defer backend.Close()

	backendURL, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatal(err)
	}
	server := newTestWorkspaceServer(t, testWorkspaceServerOptions{
		runtimeState:     defaultRuntimeState(),
		backendURL:       backendURL,
		backendTransport: backend.Client().Transport,
	})

	resp := doRequest(t, server, http.MethodDelete, workspaceProxyPath("team-a", "/api/v1/namespaces/demo"), nil, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got %d", resp.StatusCode)
	}
}

func TestWorkspaceNamespaceGetHonorsWorkspaceScope(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		switch req.URL.Path {
		case "/api/v1/namespaces/demo":
			_ = json.NewEncoder(rw).Encode(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "demo"}})
		case "/api/v1/namespaces":
			_ = json.NewEncoder(rw).Encode(&corev1.NamespaceList{})
		default:
			rw.WriteHeader(http.StatusNotFound)
		}
	}))
	defer backend.Close()

	backendURL, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatal(err)
	}
	server := newTestWorkspaceServer(t, testWorkspaceServerOptions{
		runtimeState:     defaultRuntimeState(),
		backendURL:       backendURL,
		backendTransport: backend.Client().Transport,
	})

	resp := doRequest(t, server, http.MethodGet, workspaceProxyPath("team-a", "/api/v1/namespaces/demo"), nil, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		payload, _ := io.ReadAll(resp.Body)
		t.Fatalf("got %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
}

func TestWorkspaceForbiddenContract(t *testing.T) {
	server := newTestWorkspaceServer(t, testWorkspaceServerOptions{
		runtimeState: defaultRuntimeState(),
		authorizer: func(_ string, info *workspaceRequestInfo) error {
			if info.Namespace == "forbidden" {
				return apierrors.NewForbidden(corev1.Resource("pods"), info.Name, fmt.Errorf("workspace authorization denied"))
			}
			return nil
		},
	})

	resp := doRequest(t, server, http.MethodGet, workspaceProxyPath("team-a", "/api/v1/namespaces/forbidden/pods/demo"), nil, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("got %d", resp.StatusCode)
	}
	status := decodeStatus(t, resp.Body)
	if status.Reason != metav1.StatusReasonForbidden {
		t.Fatalf("got reason %q", status.Reason)
	}
}

func TestWorkspacePodLiveMissingTargetReturnsNotFound(t *testing.T) {
	server := newTestWorkspaceServer(t, testWorkspaceServerOptions{runtimeState: &testRuntimeState{}})

	resp := doRequest(t, server, http.MethodGet, workspaceProxyPath("team-a", "/api/v1/namespaces/default/pods/demo/logs"), nil, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("got %d", resp.StatusCode)
	}
}

func TestWorkspacePodLiveAmbiguityReturnsConflict(t *testing.T) {
	state := defaultRuntimeState()
	state.targets = []live.Target{{Cluster: "member-a", Namespace: "default", Name: "demo"}, {Cluster: "member-b", Namespace: "default", Name: "demo"}}
	server := newTestWorkspaceServer(t, testWorkspaceServerOptions{runtimeState: state})

	resp := doRequest(t, server, http.MethodGet, workspaceProxyPath("team-a", "/api/v1/namespaces/default/pods/demo/logs"), nil, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("got %d", resp.StatusCode)
	}
}

func TestWorkspaceWatchContract(t *testing.T) {
	server := newTestWorkspaceServer(t, testWorkspaceServerOptions{runtimeState: defaultRuntimeState()})

	resp := doRequest(t, server, http.MethodGet, workspaceProxyPath("team-a", "/api/v1/namespaces/default/pods?watch=1"), nil, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pod watch got %d", resp.StatusCode)
	}

	resp = doRequest(t, server, http.MethodGet, workspaceProxyPath("team-a", "/api/v1/nodes?watch=1"), nil, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("node watch got %d", resp.StatusCode)
	}
}

type testWorkspaceServerOptions struct {
	backend          *recordingBackend
	backendURL       *url.URL
	backendTransport http.RoundTripper
	runtimeState     *testRuntimeState
	authorizer       func(string, *workspaceRequestInfo) error
}

type recordingBackend struct {
	server      *httptest.Server
	lastRequest *http.Request
	lastBody    []byte
}

func newRecordingBackend(t *testing.T) *recordingBackend {
	t.Helper()
	backend := &recordingBackend{}
	backend.server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		_ = req.Body.Close()
		backend.lastRequest = req.Clone(req.Context())
		backend.lastBody = body
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)
		_, _ = rw.Write(body)
	}))
	t.Cleanup(backend.server.Close)
	return backend
}

func (b *recordingBackend) url(t *testing.T) *url.URL {
	t.Helper()
	u, err := url.Parse(b.server.URL)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func (b *recordingBackend) decodeLastObject(t *testing.T) *appsv1.Deployment {
	t.Helper()
	deployment := &appsv1.Deployment{}
	if err := json.Unmarshal(b.lastBody, deployment); err != nil {
		t.Fatal(err)
	}
	return deployment
}

type testRuntimeState struct {
	snapshot index.Snapshot
	watcher  watch.Interface
	targets  []live.Target
}

func defaultRuntimeState() *testRuntimeState {
	return &testRuntimeState{
		snapshot: index.Snapshot{
			ResourceVersion: "17",
			Pods: []index.PodSnapshot{{
				Object:   &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"}},
				Clusters: []string{"member-a"},
			}},
			Events: []index.EventSnapshot{{
				Object:         &corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: "demo.123", Namespace: "default"}},
				InvolvedObject: corev1.ObjectReference{Name: "demo", Namespace: "default", Kind: "Pod"},
			}},
		},
		watcher: watch.NewFake(),
		targets: []live.Target{{Cluster: "member-a", Namespace: "default", Name: "demo"}},
	}
}

func (s *testRuntimeState) Snapshot(context.Context) (*index.Snapshot, error) {
	snapshot := s.snapshot
	return &snapshot, nil
}

func (s *testRuntimeState) Watch(context.Context, index.WatchOptions) (watch.Interface, error) {
	if s.watcher != nil {
		return s.watcher, nil
	}
	return watch.NewFake(), nil
}

func (s *testRuntimeState) LiveTargets(live.Request) ([]live.Target, error) {
	out := make([]live.Target, len(s.targets))
	copy(out, s.targets)
	return out, nil
}

type fakeLiveConnector struct{}

func (fakeLiveConnector) Connect(_ context.Context, target live.Target, req live.Request, _ runtime.Object, _ rest.Responder) (http.Handler, error) {
	return http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(rw, target.Cluster+"/"+req.Subresource)
	}), nil
}

func newTestWorkspaceServer(t *testing.T, opts testWorkspaceServerOptions) *httptest.Server {
	t.Helper()

	client := fakeclientset.NewClientset()
	sharedInformer := informers.NewSharedInformerFactory(client, 0)
	cfg := &completedConfig{
		ExtraConfig: &ExtraConfig{
			RuntimeState:      opts.runtimeState,
			LiveResolver:      live.NewResolver(opts.runtimeState),
			LiveConnector:     fakeLiveConnector{},
			RequestAuthorizer: opts.authorizer,
		},
		GenericConfig: (&genericapiserver.Config{
			RESTOptionsGetter:          generic.RESTOptions{},
			Serializer:                 workspacescheme.Codecs,
			LoopbackClientConfig:       &restclient.Config{},
			EquivalentResourceRegistry: runtime.NewEquivalentResourceRegistry(),
			BuildHandlerChainFunc: func(apiHandler http.Handler, c *genericapiserver.Config) http.Handler {
				return genericapifilters.WithRequestInfo(apiHandler, c.RequestInfoResolver)
			},
			OpenAPIConfig:    genericapiserver.DefaultOpenAPIConfig(generatedopenapi.GetOpenAPIDefinitions, openapi.NewDefinitionNamer(workspacescheme.Scheme)),
			OpenAPIV3Config:  genericapiserver.DefaultOpenAPIV3Config(generatedopenapi.GetOpenAPIDefinitions, openapi.NewDefinitionNamer(workspacescheme.Scheme)),
			ExternalAddress:  "10.0.0.0:10000",
			EffectiveVersion: compatibility.DefaultBuildEffectiveVersion(),
		}).Complete(sharedInformer),
	}
	if opts.backend != nil {
		cfg.ExtraConfig.DesiredStateBackendURL = opts.backend.url(t)
		cfg.ExtraConfig.DesiredStateTransport = opts.backend.server.Client().Transport
	}
	if opts.backendURL != nil {
		cfg.ExtraConfig.DesiredStateBackendURL = opts.backendURL
		cfg.ExtraConfig.DesiredStateTransport = opts.backendTransport
	}

	server, err := cfg.New()
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server.GenericAPIServer.Handler)
	t.Cleanup(func() {
		httpServer.Close()
	})

	return httpServer
}

func discoverWorkspaceResources(t *testing.T, server *httptest.Server) []metav1.APIResource {
	t.Helper()
	resourceList := metav1.APIResourceList{}
	decodeResponse(t, server, "/apis/"+workspacev1alpha1.GroupName+"/"+workspacev1alpha1.GroupVersion.Version, &resourceList)
	return resourceList.APIResources
}

func discoverAPIResources(t *testing.T, server *httptest.Server, requestPath string) []metav1.APIResource {
	t.Helper()
	resourceList := metav1.APIResourceList{}
	decodeResponse(t, server, requestPath, &resourceList)
	return resourceList.APIResources
}

func workspaceProxyPath(workspaceName, suffix string) string {
	return "/apis/" + workspacev1alpha1.GroupName + "/" + workspacev1alpha1.GroupVersion.Version + "/workspaces/" + workspaceName + "/proxy" + suffix
}

func assertWorkspaceWriteContract(t *testing.T, requestPath, wantBackendPath string, object map[string]any) {
	t.Helper()

	backend := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("got method %q", req.Method)
		}
		if req.URL.Path != wantBackendPath {
			t.Fatalf("got path %q", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		_ = req.Body.Close()

		forwarded := map[string]any{}
		if err := json.Unmarshal(body, &forwarded); err != nil {
			t.Fatal(err)
		}
		assertForwardedWorkspaceMetadata(t, forwarded)

		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(rw).Encode(forwarded)
	}))
	defer backend.Close()

	backendURL, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatal(err)
	}
	server := newTestWorkspaceServer(t, testWorkspaceServerOptions{
		runtimeState:     defaultRuntimeState(),
		backendURL:       backendURL,
		backendTransport: backend.Client().Transport,
	})

	body, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}

	resp := doJSONRequest(t, server, http.MethodPost, requestPath, body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("got %d", resp.StatusCode)
	}
}

func assertForwardedWorkspaceMetadata(t *testing.T, obj map[string]any) {
	t.Helper()
	metadata, ok := obj["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("metadata missing: %#v", obj)
	}
	annotations, ok := metadata["annotations"].(map[string]any)
	if !ok {
		t.Fatalf("annotations missing: %#v", metadata)
	}
	if annotations[facade.AnnotationWorkspaceName] != "team-a" {
		t.Fatalf("missing workspace annotation: %#v", annotations)
	}
	if annotations[facade.AnnotationWorkspaceResource] == "" {
		t.Fatalf("missing workspace resource annotation: %#v", annotations)
	}
	labels, ok := metadata["labels"].(map[string]any)
	if !ok {
		t.Fatalf("labels missing: %#v", metadata)
	}
	if labels[facade.LabelWorkspaceName] != "team-a" {
		t.Fatalf("missing workspace label: %#v", labels)
	}
}

func discoverAPIGroups(t *testing.T, server *httptest.Server, requestPath string) []metav1.APIGroup {
	t.Helper()
	groupList := metav1.APIGroupList{}
	decodeResponse(t, server, requestPath, &groupList)
	return groupList.Groups
}

func containsGroup(groups []metav1.APIGroup, name string) bool {
	for _, group := range groups {
		if group.Name == name {
			return true
		}
	}
	return false
}

func doJSONRequest(t *testing.T, server *httptest.Server, method, requestPath string, body []byte) *http.Response {
	t.Helper()
	return doRequest(t, server, method, requestPath, body, "application/json")
}

func doRequest(t *testing.T, server *httptest.Server, method, requestPath string, body []byte, contentType string) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, server.URL+requestPath, reader)
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func decodeResponse(t *testing.T, server *httptest.Server, requestPath string, into any) {
	t.Helper()
	resp := doRequest(t, server, http.MethodGet, requestPath, nil, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s returned %d", requestPath, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
		t.Fatal(err)
	}
}

func decodeStatus(t *testing.T, body io.Reader) metav1.Status {
	t.Helper()
	status := metav1.Status{}
	if err := json.NewDecoder(body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

func assertStatusCode(t *testing.T, server *httptest.Server, requestPath string, want int) {
	t.Helper()
	resp := doRequest(t, server, http.MethodGet, requestPath, nil, "")
	defer resp.Body.Close()
	if resp.StatusCode != want {
		payload, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET %s returned %d, want %d: %s", requestPath, resp.StatusCode, want, strings.TrimSpace(string(payload)))
	}
}

func containsResource(resources []metav1.APIResource, name string) bool {
	for _, resource := range resources {
		if resource.Name == name {
			return true
		}
	}
	return false
}

func TestWorkspacePodLiveSelectionReturnsChosenTarget(t *testing.T) {
	state := defaultRuntimeState()
	state.targets = []live.Target{{Cluster: "member-a", Namespace: "default", Name: "demo"}, {Cluster: "member-b", Namespace: "default", Name: "demo"}}
	server := newTestWorkspaceServer(t, testWorkspaceServerOptions{runtimeState: state})

	resp := doRequest(t, server, http.MethodGet, workspaceProxyPath("team-a", "/api/v1/namespaces/default/pods/demo/logs?targetCluster=member-b"), nil, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if strings.TrimSpace(string(body)) != "member-b/logs" {
		t.Fatalf("got body %q", strings.TrimSpace(string(body)))
	}
}

func TestWorkspacePodLiveInspectionReportsCandidates(t *testing.T) {
	state := defaultRuntimeState()
	state.targets = []live.Target{{Cluster: "member-a", Namespace: "default", Name: "demo"}, {Cluster: "member-b", Namespace: "default", Name: "demo"}}
	server := newTestWorkspaceServer(t, testWorkspaceServerOptions{runtimeState: state})

	resp := doRequest(t, server, http.MethodGet, workspaceProxyPath("team-a", "/api/v1/namespaces/default/pods/demo/logs?inspect=1"), nil, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got %d", resp.StatusCode)
	}
	inspection := live.Inspection{}
	if err := json.NewDecoder(resp.Body).Decode(&inspection); err != nil {
		t.Fatal(err)
	}
	if len(inspection.Candidates) != 2 {
		t.Fatalf("got %d candidates", len(inspection.Candidates))
	}
}

func TestWorkspacePlacementViewReturnsDebugState(t *testing.T) {
	state := defaultRuntimeState()
	state.targets = []live.Target{{Cluster: "member-a", Namespace: "default", Name: "demo"}, {Cluster: "member-b", Namespace: "default", Name: "demo"}}
	server := newTestWorkspaceServer(t, testWorkspaceServerOptions{runtimeState: state})

	resp := doRequest(t, server, http.MethodGet, "/apis/"+workspacev1alpha1.GroupName+"/"+workspacev1alpha1.GroupVersion.Version+"/placementviews/default.demo", nil, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got %d", resp.StatusCode)
	}
	view := workspacev1alpha1.PlacementView{}
	if err := json.NewDecoder(resp.Body).Decode(&view); err != nil {
		t.Fatal(err)
	}
	if !view.Status.AmbiguousLiveTarget {
		t.Fatal("expected ambiguous live target")
	}
	if len(view.Status.EligibleClusters) != 2 {
		t.Fatalf("got %d eligible clusters", len(view.Status.EligibleClusters))
	}
}

func apiResourceByName(resources []metav1.APIResource, name string) (metav1.APIResource, bool) {
	for _, resource := range resources {
		if resource.Name == name {
			return resource, true
		}
	}
	return metav1.APIResource{}, false
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
