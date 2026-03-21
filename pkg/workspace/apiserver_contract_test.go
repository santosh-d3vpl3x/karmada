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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/openapi"
	"k8s.io/apiserver/pkg/registry/generic"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/util/compatibility"
	"k8s.io/client-go/informers"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	restclient "k8s.io/client-go/rest"

	workspacescheme "github.com/karmada-io/karmada/pkg/apis/workspace/scheme"
	workspacev1alpha1 "github.com/karmada-io/karmada/pkg/apis/workspace/v1alpha1"
	generatedopenapi "github.com/karmada-io/karmada/pkg/generated/openapi"
)

func TestWorkspaceAPIServerDiscoveryAdvertisesOnlyRootResources(t *testing.T) {
	server := newTestWorkspaceServer(t)

	resources := discoverWorkspaceResources(t, server)
	if len(resources) != 2 {
		t.Fatalf("got %d resources", len(resources))
	}
	if !hasResource(resources, workspacev1alpha1.ResourcePluralWorkspace) {
		t.Fatalf("expected %s in discovery", workspacev1alpha1.ResourcePluralWorkspace)
	}
	if !hasResource(resources, workspacev1alpha1.ResourcePluralPlacementView) {
		t.Fatalf("expected %s in discovery", workspacev1alpha1.ResourcePluralPlacementView)
	}
	if hasResource(resources, "pods") {
		t.Fatal("did not expect pods in phase-1 root discovery before projected resources exist")
	}
}

func TestWorkspaceAPIServerDiscoveryDoesNotAdvertiseReadableRootVerbs(t *testing.T) {
	server := newTestWorkspaceServer(t)

	resources := discoverWorkspaceResources(t, server)
	for _, name := range []string{workspacev1alpha1.ResourcePluralWorkspace, workspacev1alpha1.ResourcePluralPlacementView} {
		resource := mustFindResource(t, resources, name)
		if hasVerb(resource.Verbs, "get") {
			t.Fatalf("did not expect get on %s", name)
		}
		if hasVerb(resource.Verbs, "list") {
			t.Fatalf("did not expect list on %s", name)
		}
		if hasVerb(resource.Verbs, "watch") {
			t.Fatalf("did not expect watch on %s", name)
		}
	}
}

func TestWorkspaceAPIServerUnsupportedRootReadsReturnNotFound(t *testing.T) {
	server := newTestWorkspaceServer(t)

	assertStatusCode(t, server, "/apis/"+workspacev1alpha1.GroupName+"/"+workspacev1alpha1.GroupVersion.Version+"/workspaces/team-a", http.StatusNotFound)
	assertStatusCode(t, server, "/apis/"+workspacev1alpha1.GroupName+"/"+workspacev1alpha1.GroupVersion.Version+"/placementviews/demo", http.StatusNotFound)
	assertStatusCode(t, server, "/apis/"+workspacev1alpha1.GroupName+"/"+workspacev1alpha1.GroupVersion.Version+"/pods", http.StatusNotFound)
}

func newTestWorkspaceServer(t *testing.T) *httptest.Server {
	t.Helper()

	client := fakeclientset.NewClientset()
	sharedInformer := informers.NewSharedInformerFactory(client, 0)
	cfg := &completedConfig{
		ExtraConfig: &ExtraConfig{},
		GenericConfig: (&genericapiserver.Config{
			RESTOptionsGetter:          generic.RESTOptions{},
			Serializer:                 workspacescheme.Codecs,
			LoopbackClientConfig:       &restclient.Config{},
			EquivalentResourceRegistry: runtime.NewEquivalentResourceRegistry(),
			BuildHandlerChainFunc: func(apiHandler http.Handler, _ *genericapiserver.Config) http.Handler {
				return apiHandler
			},
			OpenAPIConfig:    genericapiserver.DefaultOpenAPIConfig(generatedopenapi.GetOpenAPIDefinitions, openapi.NewDefinitionNamer(workspacescheme.Scheme)),
			OpenAPIV3Config:  genericapiserver.DefaultOpenAPIV3Config(generatedopenapi.GetOpenAPIDefinitions, openapi.NewDefinitionNamer(workspacescheme.Scheme)),
			ExternalAddress:  "10.0.0.0:10000",
			EffectiveVersion: compatibility.DefaultBuildEffectiveVersion(),
		}).Complete(sharedInformer),
	}

	server, err := cfg.New()
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server.GenericAPIServer.UnprotectedHandler())
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

func decodeResponse(t *testing.T, server *httptest.Server, requestPath string, into any) {
	t.Helper()

	resp, err := server.Client().Get(server.URL + requestPath)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s returned %d", requestPath, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
		t.Fatal(err)
	}
}

func assertStatusCode(t *testing.T, server *httptest.Server, requestPath string, want int) {
	t.Helper()

	resp, err := server.Client().Get(server.URL + requestPath)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != want {
		t.Fatalf("GET %s returned %d, want %d", requestPath, resp.StatusCode, want)
	}
}

func hasResource(resources []metav1.APIResource, name string) bool {
	for _, resource := range resources {
		if resource.Name == name {
			return true
		}
	}
	return false
}

func mustFindResource(t *testing.T, resources []metav1.APIResource, name string) metav1.APIResource {
	t.Helper()

	for _, resource := range resources {
		if resource.Name == name {
			return resource
		}
	}
	t.Fatal(fmt.Sprintf("resource %s missing from discovery", name))
	return metav1.APIResource{}
}

func hasVerb(verbs metav1.Verbs, want string) bool {
	for _, verb := range verbs {
		if verb == want {
			return true
		}
	}
	return false
}
