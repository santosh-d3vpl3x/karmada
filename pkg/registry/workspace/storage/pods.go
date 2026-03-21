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
	"net/http"
	"sort"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	genericrequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"

	"github.com/karmada-io/karmada/pkg/workspace/index"
	"github.com/karmada-io/karmada/pkg/workspace/live"
	"github.com/karmada-io/karmada/pkg/workspace/projection"
)

// PodREST is the live subresource wrapper used by the workspace API.
type PodREST struct {
	*PodLiveREST
}

// NewPodREST returns a pod storage wrapper with live subresource support.
func NewPodREST(resolver live.Resolver, connector LiveConnector) *PodREST {
	return &PodREST{PodLiveREST: NewPodLiveREST(resolver, connector)}
}

// New returns an empty Pod object.
func (r *PodREST) New() runtime.Object {
	return r.PodLiveREST.New()
}

// NamespaceScoped reports that pods are namespace-scoped.
func (r *PodREST) NamespaceScoped() bool {
	return r.PodLiveREST.NamespaceScoped()
}

// Destroy cleans up on shutdown.
func (r *PodREST) Destroy() {
	r.PodLiveREST.Destroy()
}

// GetSingularName returns the singular resource name for pods.
func (r *PodREST) GetSingularName() string {
	return r.PodLiveREST.GetSingularName()
}

// ConnectMethods returns the methods accepted by pod live subresources.
func (r *PodREST) ConnectMethods() []string {
	return r.PodLiveREST.ConnectMethods()
}

// NewConnectOptions uses request query parameters directly, so no typed connect options are required.
func (r *PodREST) NewConnectOptions() (runtime.Object, bool, string) {
	return r.PodLiveREST.NewConnectOptions()
}

// Connect resolves the pod target and returns a connector handler for the chosen backing cluster.
func (r *PodREST) Connect(ctx context.Context, id string, options runtime.Object, responder rest.Responder) (http.Handler, error) {
	return r.PodLiveREST.Connect(ctx, id, options, responder)
}

// projectedRuntimeIndex is the injectable runtime feed used by projected pod and event storage.
type projectedRuntimeIndex interface {
	Snapshot(context.Context) (*index.Snapshot, error)
	Watch(context.Context, index.WatchOptions) (watch.Interface, error)
}

// ProjectedPodREST serves read-only workspace pod projections from the index snapshot.
type ProjectedPodREST struct {
	index          projectedRuntimeIndex
	tableConvertor rest.TableConvertor
}

var _ rest.Scoper = &ProjectedPodREST{}
var _ rest.Storage = &ProjectedPodREST{}
var _ rest.Getter = &ProjectedPodREST{}
var _ rest.Lister = &ProjectedPodREST{}
var _ rest.Watcher = &ProjectedPodREST{}
var _ rest.SingularNameProvider = &ProjectedPodREST{}
var _ rest.TableConvertor = &ProjectedPodREST{}

// NewProjectedPodREST returns a read-only projected pod storage backed by workspace index state.
func NewProjectedPodREST(index projectedRuntimeIndex) *ProjectedPodREST {
	return &ProjectedPodREST{
		index:          index,
		tableConvertor: rest.NewDefaultTableConvertor(corev1.SchemeGroupVersion.WithResource("pods").GroupResource()),
	}
}

// New returns an empty Pod object.
func (r *ProjectedPodREST) New() runtime.Object {
	return &corev1.Pod{}
}

// NewList returns an empty PodList object.
func (r *ProjectedPodREST) NewList() runtime.Object {
	return &corev1.PodList{}
}

// NamespaceScoped returns true because pods are namespaced.
func (r *ProjectedPodREST) NamespaceScoped() bool {
	return true
}

// Destroy cleans up on shutdown.
func (r *ProjectedPodREST) Destroy() {}

// GetSingularName returns the singular resource name for pods.
func (r *ProjectedPodREST) GetSingularName() string {
	return "pod"
}

// ConvertToTable delegates to the default pod table convertor.
func (r *ProjectedPodREST) ConvertToTable(ctx context.Context, object runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	return r.tableConvertor.ConvertToTable(ctx, object, tableOptions)
}

// Get returns one projected pod by workspace-visible name.
func (r *ProjectedPodREST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	_ = options

	snapshot, err := r.snapshot(ctx)
	if err != nil {
		return nil, err
	}

	namespace := namespaceFromContext(ctx)
	for _, projected := range projectedPodsFromSnapshot(snapshot, namespace, nil) {
		if projected.Name == name {
			return projected.DeepCopy(), nil
		}
	}

	return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "pods"}, name)
}

// List returns the projected pods visible from the workspace index.
func (r *ProjectedPodREST) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	snapshot, err := r.snapshot(ctx)
	if err != nil {
		return nil, err
	}

	namespace := namespaceFromContext(ctx)
	items := projectedPodsFromSnapshot(snapshot, namespace, options)
	list := &corev1.PodList{}
	if snapshot != nil {
		list.ResourceVersion = snapshot.ResourceVersion
	}
	for _, item := range items {
		list.Items = append(list.Items, *item)
	}
	return list, nil
}

// Watch returns a watch sourced from the workspace index instead of backing-cluster fan-out.
func (r *ProjectedPodREST) Watch(ctx context.Context, options *metainternalversion.ListOptions) (watch.Interface, error) {
	if r.index == nil {
		return watch.NewFake(), nil
	}

	resourceVersion := ""
	if options != nil {
		resourceVersion = options.ResourceVersion
	}
	return r.index.Watch(ctx, index.WatchOptions{
		Namespace:       namespaceFromContext(ctx),
		ResourceVersion: resourceVersion,
	})
}

func (r *ProjectedPodREST) snapshot(ctx context.Context) (*index.Snapshot, error) {
	if r.index == nil {
		return &index.Snapshot{}, nil
	}
	return r.index.Snapshot(ctx)
}

func projectedPodsFromSnapshot(snapshot *index.Snapshot, namespace string, options *metainternalversion.ListOptions) []*corev1.Pod {
	if snapshot == nil {
		return nil
	}

	items := make([]*corev1.Pod, 0, len(snapshot.Pods))
	for _, item := range snapshot.Pods {
		if item.Object == nil {
			continue
		}
		if !namespaceMatches(namespace, item.Object.Namespace) {
			continue
		}
		projected := projection.ProjectPod(item.Object, item.Clusters)
		if projected == nil {
			continue
		}
		if options != nil && !matchesListOptions(projected.Namespace, projected.Name, projected.Labels, options) {
			continue
		}
		if snapshot.ResourceVersion != "" {
			projected.ResourceVersion = snapshot.ResourceVersion
		}
		items = append(items, projected)
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Namespace != items[j].Namespace {
			return items[i].Namespace < items[j].Namespace
		}
		return items[i].Name < items[j].Name
	})
	return items
}

func namespaceFromContext(ctx context.Context) string {
	namespace, _ := genericrequest.NamespaceFrom(ctx)
	return namespace
}

func namespaceMatches(requested, objectNamespace string) bool {
	return requested == "" || requested == objectNamespace
}

func matchesListOptions(namespace, name string, objectLabels map[string]string, options *metainternalversion.ListOptions) bool {
	if options == nil {
		return true
	}
	if options.LabelSelector != nil && !options.LabelSelector.Matches(labels.Set(objectLabels)) {
		return false
	}
	if options.FieldSelector != nil {
		if !options.FieldSelector.Matches(fields.Set{
			"metadata.name":      name,
			"metadata.namespace": namespace,
		}) {
			return false
		}
	}
	return true
}
