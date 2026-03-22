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

package storage

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"

	"github.com/karmada-io/karmada/pkg/workspace/index"
	"github.com/karmada-io/karmada/pkg/workspace/live"
)

// Storage bundles the phase-1 root resources served by the workspace API group.
type Storage struct {
	Workspaces     rest.Storage
	PlacementViews rest.Storage
}

type placementViewDependencies interface {
	RuntimeState() index.State
	LiveResolver() live.Resolver
}

// NewStorage returns the root storages installed for the workspace API group.
func NewStorage(_ *runtime.Scheme, _ generic.RESTOptionsGetter, proxyHandlers ...WorkspaceProxyHandler) (*Storage, error) {
	var proxyHandler WorkspaceProxyHandler
	var placementIndex index.State
	var placementResolver live.Resolver
	if len(proxyHandlers) > 0 {
		proxyHandler = proxyHandlers[0]
		if deps, ok := proxyHandler.(placementViewDependencies); ok {
			placementIndex = deps.RuntimeState()
			placementResolver = deps.LiveResolver()
		}
	}

	return &Storage{
		Workspaces:     NewWorkspaceREST(proxyHandler),
		PlacementViews: NewPlacementViewREST(placementIndex, placementResolver),
	}, nil
}
