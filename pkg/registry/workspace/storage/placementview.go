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
	"k8s.io/apiserver/pkg/registry/rest"

	workspacev1alpha1 "github.com/karmada-io/karmada/pkg/apis/workspace/v1alpha1"
)

// PlacementViewREST implements the root placementviews resource surface.
type PlacementViewREST struct{}

var _ rest.Scoper = &PlacementViewREST{}
var _ rest.Storage = &PlacementViewREST{}
var _ rest.SingularNameProvider = &PlacementViewREST{}

// NewPlacementViewREST returns a root REST storage for placement views.
func NewPlacementViewREST() *PlacementViewREST {
	return &PlacementViewREST{}
}

// New returns an empty PlacementView object.
func (r *PlacementViewREST) New() runtime.Object {
	return &workspacev1alpha1.PlacementView{}
}

// NamespaceScoped returns false because PlacementView is cluster scoped.
func (r *PlacementViewREST) NamespaceScoped() bool {
	return workspacev1alpha1.ResourceNamespaceScopedPlacementView
}

// Destroy cleans up on shutdown.
func (r *PlacementViewREST) Destroy() {}

// GetSingularName returns the singular resource name for placement views.
func (r *PlacementViewREST) GetSingularName() string {
	return workspacev1alpha1.ResourceSingularPlacementView
}
