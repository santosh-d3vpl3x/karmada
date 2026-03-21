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

// WorkspaceREST implements the root workspace resource surface.
type WorkspaceREST struct{}

var _ rest.Scoper = &WorkspaceREST{}
var _ rest.Storage = &WorkspaceREST{}
var _ rest.SingularNameProvider = &WorkspaceREST{}

// NewWorkspaceREST returns a root REST storage for workspaces.
func NewWorkspaceREST() *WorkspaceREST {
	return &WorkspaceREST{}
}

// New returns an empty Workspace object.
func (r *WorkspaceREST) New() runtime.Object {
	return &workspacev1alpha1.Workspace{}
}

// NamespaceScoped returns false because Workspace is cluster scoped.
func (r *WorkspaceREST) NamespaceScoped() bool {
	return workspacev1alpha1.ResourceNamespaceScopedWorkspace
}

// Destroy cleans up on shutdown.
func (r *WorkspaceREST) Destroy() {}

// GetSingularName returns the singular resource name for workspaces.
func (r *WorkspaceREST) GetSingularName() string {
	return workspacev1alpha1.ResourceSingularWorkspace
}
