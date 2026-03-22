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
	"net/http"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/openapi"
	"k8s.io/apiserver/pkg/registry/generic"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/util/compatibility"
	"k8s.io/client-go/informers"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	restclient "k8s.io/client-go/rest"

	workspacescheme "github.com/karmada-io/karmada/pkg/apis/workspace/scheme"
	generatedopenapi "github.com/karmada-io/karmada/pkg/generated/openapi"
)

func TestWorkspaceAPIServerInstallsWorkspaceGroup(t *testing.T) {
	client := fakeclientset.NewClientset()
	sharedInformer := informers.NewSharedInformerFactory(client, 0)

	cfg := &completedConfig{
		ExtraConfig: &ExtraConfig{},
		GenericConfig: (&genericapiserver.Config{
			RESTOptionsGetter: generic.RESTOptions{},
			Serializer: runtime.NewSimpleNegotiatedSerializer(runtime.SerializerInfo{
				MediaType: runtime.ContentTypeJSON,
			}),
			LoopbackClientConfig:       &restclient.Config{},
			EquivalentResourceRegistry: runtime.NewEquivalentResourceRegistry(),
			BuildHandlerChainFunc: func(http.Handler, *genericapiserver.Config) http.Handler {
				return nil
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
	if server.GenericAPIServer == nil {
		t.Fatal("expected generic apiserver")
	}
}

func TestWorkspaceDiscoveryGroupVersionsFollowSupportCatalog(t *testing.T) {
	expected := []schema.GroupVersion{
		appsv1.SchemeGroupVersion,
		autoscalingv2.SchemeGroupVersion,
		batchv1.SchemeGroupVersion,
		networkingv1.SchemeGroupVersion,
		policyv1.SchemeGroupVersion,
		rbacv1.SchemeGroupVersion,
	}

	if actual := discoveryGroupVersions(); !reflect.DeepEqual(actual, expected) {
		t.Fatalf("unexpected discovery group versions: %#v", actual)
	}
}
