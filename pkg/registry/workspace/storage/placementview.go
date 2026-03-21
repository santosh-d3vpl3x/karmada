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
	"context"

	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/registry/rest"

	workspacev1alpha1 "github.com/karmada-io/karmada/pkg/apis/workspace/v1alpha1"
)

// PlacementViewREST implements the root placementviews resource surface.
type PlacementViewREST struct {
	tableConvertor rest.TableConvertor
}

var _ rest.Scoper = &PlacementViewREST{}
var _ rest.Storage = &PlacementViewREST{}
var _ rest.Getter = &PlacementViewREST{}
var _ rest.Lister = &PlacementViewREST{}
var _ rest.SingularNameProvider = &PlacementViewREST{}

// NewPlacementViewREST returns a root REST storage for placement views.
func NewPlacementViewREST() *PlacementViewREST {
	return &PlacementViewREST{tableConvertor: rest.NewDefaultTableConvertor(placementViewResource().GroupResource())}
}

func placementViewResource() schema.GroupVersionResource {
	return workspacev1alpha1.SchemeGroupVersion.WithResource(workspacev1alpha1.ResourcePluralPlacementView)
}

// New returns an empty PlacementView object.
func (r *PlacementViewREST) New() runtime.Object {
	return &workspacev1alpha1.PlacementView{}
}

// NewList returns an empty PlacementViewList object.
func (r *PlacementViewREST) NewList() runtime.Object {
	return &workspacev1alpha1.PlacementViewList{}
}

// Get returns the phase-1 unsupported-verb error until placement projection is implemented.
func (r *PlacementViewREST) Get(context.Context, string, *metav1.GetOptions) (runtime.Object, error) {
	return nil, NewUnsupportedVerbError(workspacev1alpha1.ResourcePluralPlacementView, "get")
}

// List returns the phase-1 unsupported-verb error until placement projection is implemented.
func (r *PlacementViewREST) List(context.Context, *metainternalversion.ListOptions) (runtime.Object, error) {
	return nil, NewUnsupportedVerbError(workspacev1alpha1.ResourcePluralPlacementView, "list")
}

// ConvertToTable delegates to the default table convertor.
func (r *PlacementViewREST) ConvertToTable(ctx context.Context, object runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	return r.tableConvertor.ConvertToTable(ctx, object, tableOptions)
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
