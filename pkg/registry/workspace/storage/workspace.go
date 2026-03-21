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
	"fmt"
	"net/http"
	"path"

	"k8s.io/apimachinery/pkg/runtime"
	genericrequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"

	workspacev1alpha1 "github.com/karmada-io/karmada/pkg/apis/workspace/v1alpha1"
)

var workspaceProxyConnectMethods = []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions}

// WorkspaceProxyHandler serves the nested workspace endpoint under the root proxy subresource.
type WorkspaceProxyHandler interface {
	ConnectWorkspace(context.Context, string, string, rest.Responder) (http.Handler, error)
}

// WorkspaceREST implements the root workspace resource surface.
type WorkspaceREST struct {
	proxy WorkspaceProxyHandler
}

var _ rest.Scoper = &WorkspaceREST{}
var _ rest.Storage = &WorkspaceREST{}
var _ rest.SingularNameProvider = &WorkspaceREST{}
var _ rest.Connecter = &WorkspaceREST{}

// NewWorkspaceREST returns a root REST storage for workspaces.
func NewWorkspaceREST(proxy WorkspaceProxyHandler) *WorkspaceREST {
	return &WorkspaceREST{proxy: proxy}
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

// ConnectMethods returns the methods handled by the workspace proxy subresource.
func (r *WorkspaceREST) ConnectMethods() []string {
	return workspaceProxyConnectMethods
}

// NewConnectOptions uses the nested proxy path directly, so no typed connect options are required.
func (r *WorkspaceREST) NewConnectOptions() (runtime.Object, bool, string) {
	return nil, true, ""
}

// Connect delegates the nested workspace endpoint to the configured proxy handler.
func (r *WorkspaceREST) Connect(ctx context.Context, id string, _ runtime.Object, responder rest.Responder) (http.Handler, error) {
	if r.proxy == nil {
		return nil, NewUnsupportedResourceError("proxy")
	}

	info, ok := genericrequest.RequestInfoFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("no RequestInfo found in the context")
	}
	if len(info.Parts) < 3 {
		return nil, fmt.Errorf("invalid requestInfo parts: %v", info.Parts)
	}

	proxyPath := "/" + path.Join(info.Parts[3:]...)
	return r.proxy.ConnectWorkspace(ctx, id, proxyPath, responder)
}
