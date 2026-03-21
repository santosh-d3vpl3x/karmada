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
	"testing"

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
