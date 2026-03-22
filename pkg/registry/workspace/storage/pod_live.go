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
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	genericrequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"

	"github.com/karmada-io/karmada/pkg/workspace/live"
	"github.com/karmada-io/karmada/pkg/workspace/support"
)

var podLiveConnectMethods = []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions}

type liveRequestOptionsKey struct{}

type liveRequestOptions struct {
	Inspect       bool
	TargetCluster string
}

// LiveConnector proxies a uniquely resolved live request to the selected backing target.
type LiveConnector interface {
	Connect(context.Context, live.Target, live.Request, runtime.Object, rest.Responder) (http.Handler, error)
}

// PodLiveREST routes pod live subresources to a uniquely resolved backing target.
type PodLiveREST struct {
	resolver  live.Resolver
	connector LiveConnector
}

var _ rest.Scoper = &PodLiveREST{}
var _ rest.Storage = &PodLiveREST{}
var _ rest.Connecter = &PodLiveREST{}
var _ rest.SingularNameProvider = &PodLiveREST{}

// NewPodLiveREST returns a live subresource storage for pods.
func NewPodLiveREST(resolver live.Resolver, connector LiveConnector) *PodLiveREST {
	return &PodLiveREST{
		resolver:  resolver,
		connector: connector,
	}
}

// WithLiveRequestOptions stores live-routing query options on the request context.
func WithLiveRequestOptions(ctx context.Context, query url.Values) context.Context {
	if len(query) == 0 {
		return ctx
	}
	options := liveRequestOptions{
		Inspect:       truthy(query.Get("inspect")),
		TargetCluster: strings.TrimSpace(query.Get("targetCluster")),
	}
	if !options.Inspect && options.TargetCluster == "" {
		return ctx
	}
	return context.WithValue(ctx, liveRequestOptionsKey{}, options)
}

// New returns an empty Pod object.
func (r *PodLiveREST) New() runtime.Object {
	return &corev1.Pod{}
}

// NamespaceScoped returns true because pods are namespaced.
func (r *PodLiveREST) NamespaceScoped() bool {
	return true
}

// Destroy cleans up on shutdown.
func (r *PodLiveREST) Destroy() {}

// GetSingularName returns the singular resource name for pods.
func (r *PodLiveREST) GetSingularName() string {
	return "pod"
}

// ConnectMethods returns the methods accepted by pod live subresources.
func (r *PodLiveREST) ConnectMethods() []string {
	return podLiveConnectMethods
}

// NewConnectOptions uses request query parameters directly, so no typed connect options are required.
func (r *PodLiveREST) NewConnectOptions() (runtime.Object, bool, string) {
	return nil, true, ""
}

// Connect resolves the pod target and returns a connector handler for the chosen backing cluster.
func (r *PodLiveREST) Connect(ctx context.Context, id string, options runtime.Object, responder rest.Responder) (http.Handler, error) {
	info, ok := genericrequest.RequestInfoFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("no RequestInfo found in the context")
	}

	subresource := strings.ToLower(info.Subresource)
	if !support.SupportsSubresource(corev1.SchemeGroupVersion.WithResource("pods"), subresource) {
		return nil, NewUnsupportedResourceError(subresource)
	}

	request := r.liveRequest(info, id, liveRequestOptionsFromContext(ctx))
	target, err := r.resolveTarget(request)
	if err != nil {
		return nil, err
	}

	if r.connector == nil {
		return nil, fmt.Errorf("live connector is not configured")
	}
	return r.connector.Connect(ctx, target, request, options, responder)
}

// Inspect reports the eligible backing targets without selecting one implicitly.
func (r *PodLiveREST) Inspect(ctx context.Context, id string) (live.Inspection, error) {
	info, ok := genericrequest.RequestInfoFrom(ctx)
	if !ok {
		return live.Inspection{}, fmt.Errorf("no RequestInfo found in the context")
	}

	subresource := strings.ToLower(info.Subresource)
	if !support.SupportsSubresource(corev1.SchemeGroupVersion.WithResource("pods"), subresource) {
		return live.Inspection{}, NewUnsupportedResourceError(subresource)
	}
	if r.resolver == nil {
		return live.Inspection{}, fmt.Errorf("live resolver is not configured")
	}

	inspection, err := r.resolver.InspectLiveTargets(r.liveRequest(info, id, liveRequestOptionsFromContext(ctx)))
	if err != nil {
		return live.Inspection{}, err
	}
	return inspection, nil
}

func (r *PodLiveREST) liveRequest(info *genericrequest.RequestInfo, name string, options liveRequestOptions) live.Request {
	return live.Request{
		Resource:    corev1.SchemeGroupVersion.WithResource("pods"),
		Namespace:   info.Namespace,
		Name:        name,
		Subresource: strings.ToLower(info.Subresource),
		Selector: live.TargetSelector{
			Cluster: options.TargetCluster,
		},
	}
}

func (r *PodLiveREST) resolveTarget(req live.Request) (live.Target, error) {
	if r.resolver == nil {
		return live.Target{}, fmt.Errorf("live resolver is not configured")
	}

	target, err := r.resolver.ResolveLiveTarget(req)
	if err == nil {
		return target, nil
	}

	switch {
	case errors.Is(err, live.ErrNoLiveTarget):
		return live.Target{}, NewNoEligibleLiveTargetError("pods", req.Name)
	case errors.Is(err, live.ErrAmbiguousLiveTarget):
		return live.Target{}, NewAmbiguousTargetError("pods", req.Name)
	default:
		return live.Target{}, err
	}
}

func liveRequestOptionsFromContext(ctx context.Context) liveRequestOptions {
	options, _ := ctx.Value(liveRequestOptionsKey{}).(liveRequestOptions)
	return options
}

func truthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "t", "true", "y", "yes", "on":
		return true
	default:
		return false
	}
}
