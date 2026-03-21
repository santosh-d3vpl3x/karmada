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
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/util/compatibility"
	"k8s.io/klog/v2"

	workspacescheme "github.com/karmada-io/karmada/pkg/apis/workspace/scheme"
	workspacev1alpha1 "github.com/karmada-io/karmada/pkg/apis/workspace/v1alpha1"
	workspacestorage "github.com/karmada-io/karmada/pkg/registry/workspace/storage"
)

const componentName = "karmada-workspace-apiserver"

// ExtraConfig holds custom apiserver config.
type ExtraConfig struct{}

// Config defines the config for the workspace API server.
type Config struct {
	GenericConfig *genericapiserver.RecommendedConfig
	ExtraConfig   ExtraConfig
}

// APIServer contains state for the workspace apiserver.
type APIServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

type completedConfig struct {
	GenericConfig genericapiserver.CompletedConfig
	ExtraConfig   *ExtraConfig
}

// CompletedConfig embeds a private pointer that cannot be instantiated outside this package.
type CompletedConfig struct {
	*completedConfig
}

// Complete fills in any fields not set that are required to have valid data.
func (cfg *Config) Complete() CompletedConfig {
	c := completedConfig{
		GenericConfig: cfg.GenericConfig.Complete(),
		ExtraConfig:   &cfg.ExtraConfig,
	}
	c.GenericConfig.EffectiveVersion = compatibility.DefaultBuildEffectiveVersion()
	return CompletedConfig{&c}
}

var storageMapBuilder = func(optsGetter generic.RESTOptionsGetter) (map[string]rest.Storage, error) {
	storage, err := workspacestorage.NewStorage(workspacescheme.Scheme, optsGetter)
	if err != nil {
		return nil, err
	}

	return map[string]rest.Storage{
		workspacev1alpha1.ResourcePluralWorkspace:     storage.Workspaces,
		workspacev1alpha1.ResourcePluralPlacementView: storage.PlacementViews,
	}, nil
}

var apiGroupInstaller = func(server *APIServer, apiGroupInfo *genericapiserver.APIGroupInfo) error {
	return server.GenericAPIServer.InstallAPIGroup(apiGroupInfo)
}

func (c completedConfig) New() (*APIServer, error) {
	genericServer, err := c.GenericConfig.New(componentName, genericapiserver.NewEmptyDelegate())
	if err != nil {
		return nil, err
	}

	server := &APIServer{GenericAPIServer: genericServer}
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(workspacev1alpha1.GroupName, workspacescheme.Scheme, workspacescheme.ParameterCodec, workspacescheme.Codecs)

	storageMap, err := storageMapBuilder(c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		klog.Errorf("unable to create REST storage for a workspace resource due to %v, will die", err)
		return nil, err
	}
	apiGroupInfo.VersionedResourcesStorageMap[workspacev1alpha1.GroupVersion.Version] = storageMap

	if err := apiGroupInstaller(server, &apiGroupInfo); err != nil {
		return nil, err
	}
	return server, nil
}
