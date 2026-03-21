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
	"sort"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/registry/rest"

	"github.com/karmada-io/karmada/pkg/workspace/index"
	"github.com/karmada-io/karmada/pkg/workspace/projection"
)

// ProjectedEventREST serves read-only workspace event projections from the index snapshot.
type ProjectedEventREST struct {
	index          projectedRuntimeIndex
	tableConvertor rest.TableConvertor
}

var _ rest.Scoper = &ProjectedEventREST{}
var _ rest.Storage = &ProjectedEventREST{}
var _ rest.Getter = &ProjectedEventREST{}
var _ rest.Lister = &ProjectedEventREST{}
var _ rest.Watcher = &ProjectedEventREST{}
var _ rest.SingularNameProvider = &ProjectedEventREST{}
var _ rest.TableConvertor = &ProjectedEventREST{}

// NewProjectedEventREST returns a read-only projected event storage backed by workspace index state.
func NewProjectedEventREST(index projectedRuntimeIndex) *ProjectedEventREST {
	return &ProjectedEventREST{
		index:          index,
		tableConvertor: rest.NewDefaultTableConvertor(corev1.SchemeGroupVersion.WithResource("events").GroupResource()),
	}
}

// New returns an empty Event object.
func (r *ProjectedEventREST) New() runtime.Object {
	return &corev1.Event{}
}

// NewList returns an empty EventList object.
func (r *ProjectedEventREST) NewList() runtime.Object {
	return &corev1.EventList{}
}

// NamespaceScoped returns true because events are namespaced.
func (r *ProjectedEventREST) NamespaceScoped() bool {
	return true
}

// Destroy cleans up on shutdown.
func (r *ProjectedEventREST) Destroy() {}

// GetSingularName returns the singular resource name for events.
func (r *ProjectedEventREST) GetSingularName() string {
	return "event"
}

// ConvertToTable delegates to the default event table convertor.
func (r *ProjectedEventREST) ConvertToTable(ctx context.Context, object runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	return r.tableConvertor.ConvertToTable(ctx, object, tableOptions)
}

// Get returns one projected event by workspace-visible name.
func (r *ProjectedEventREST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	_ = options

	snapshot, err := r.snapshot(ctx)
	if err != nil {
		return nil, err
	}

	namespace := namespaceFromContext(ctx)
	for _, projected := range projectedEventsFromSnapshot(snapshot, namespace, nil) {
		if projected.Name == name {
			return projected.DeepCopy(), nil
		}
	}

	return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "events"}, name)
}

// List returns the projected events visible from the workspace index.
func (r *ProjectedEventREST) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	snapshot, err := r.snapshot(ctx)
	if err != nil {
		return nil, err
	}

	namespace := namespaceFromContext(ctx)
	items := projectedEventsFromSnapshot(snapshot, namespace, options)
	list := &corev1.EventList{}
	if snapshot != nil {
		list.ResourceVersion = snapshot.ResourceVersion
	}
	for _, item := range items {
		list.Items = append(list.Items, *item)
	}
	return list, nil
}

// Watch returns a watch sourced from the workspace index instead of backing-cluster fan-out.
func (r *ProjectedEventREST) Watch(ctx context.Context, options *metainternalversion.ListOptions) (watch.Interface, error) {
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

func (r *ProjectedEventREST) snapshot(ctx context.Context) (*index.Snapshot, error) {
	if r.index == nil {
		return &index.Snapshot{}, nil
	}
	return r.index.Snapshot(ctx)
}

func projectedEventsFromSnapshot(snapshot *index.Snapshot, namespace string, options *metainternalversion.ListOptions) []*corev1.Event {
	if snapshot == nil {
		return nil
	}

	items := make([]*corev1.Event, 0, len(snapshot.Events))
	for _, item := range snapshot.Events {
		if item.Object == nil {
			continue
		}
		if !namespaceMatches(namespace, item.Object.Namespace) {
			continue
		}
		projected := projection.ProjectEvent(item.Object, item.InvolvedObject)
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
