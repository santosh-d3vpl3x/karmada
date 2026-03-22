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
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/registry/rest"

	workspacev1alpha1 "github.com/karmada-io/karmada/pkg/apis/workspace/v1alpha1"
	"github.com/karmada-io/karmada/pkg/workspace/index"
	"github.com/karmada-io/karmada/pkg/workspace/live"
)

type placementViewRuntime interface {
	Snapshot(context.Context) (*index.Snapshot, error)
}

// PlacementViewREST implements the root placementviews resource surface.
type PlacementViewREST struct {
	index          placementViewRuntime
	resolver       live.Resolver
	tableConvertor rest.TableConvertor
}

var _ rest.Scoper = &PlacementViewREST{}
var _ rest.Storage = &PlacementViewREST{}
var _ rest.Getter = &PlacementViewREST{}
var _ rest.Lister = &PlacementViewREST{}
var _ rest.SingularNameProvider = &PlacementViewREST{}
var _ rest.TableConvertor = &PlacementViewREST{}

// NewPlacementViewREST returns a root REST storage for placement views.
func NewPlacementViewREST(index placementViewRuntime, resolver live.Resolver) *PlacementViewREST {
	return &PlacementViewREST{
		index:          index,
		resolver:       resolver,
		tableConvertor: rest.NewDefaultTableConvertor(schema.GroupVersionResource{Group: workspacev1alpha1.GroupName, Version: workspacev1alpha1.GroupVersion.Version, Resource: workspacev1alpha1.ResourcePluralPlacementView}.GroupResource()),
	}
}

// New returns an empty PlacementView object.
func (r *PlacementViewREST) New() runtime.Object {
	return &workspacev1alpha1.PlacementView{}
}

// NewList returns an empty PlacementViewList object.
func (r *PlacementViewREST) NewList() runtime.Object {
	return &workspacev1alpha1.PlacementViewList{}
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

// ConvertToTable delegates to the default placement view table convertor.
func (r *PlacementViewREST) ConvertToTable(ctx context.Context, object runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	return r.tableConvertor.ConvertToTable(ctx, object, tableOptions)
}

// Get returns a read-only placement/debug view for one workspace-visible object.
func (r *PlacementViewREST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	_ = options

	namespace, objectName, ok := splitPlacementViewName(name)
	if !ok {
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: workspacev1alpha1.ResourcePluralPlacementView}, name)
	}

	view, err := r.findPlacementView(ctx, namespace, objectName)
	if err != nil {
		return nil, err
	}
	if view == nil {
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: workspacev1alpha1.ResourcePluralPlacementView}, name)
	}
	return view, nil
}

// List returns the placement/debug views for projected workspace pods.
func (r *PlacementViewREST) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	_ = options

	snapshot, err := r.snapshot(ctx)
	if err != nil {
		return nil, err
	}

	list := &workspacev1alpha1.PlacementViewList{}
	if snapshot != nil {
		list.ResourceVersion = snapshot.ResourceVersion
	}
	for _, pod := range sortedSnapshotPods(snapshot) {
		view, err := r.buildPlacementView(pod)
		if err != nil {
			return nil, err
		}
		if view != nil {
			list.Items = append(list.Items, *view)
		}
	}
	return list, nil
}

func (r *PlacementViewREST) findPlacementView(ctx context.Context, namespace, name string) (*workspacev1alpha1.PlacementView, error) {
	snapshot, err := r.snapshot(ctx)
	if err != nil {
		return nil, err
	}
	for _, pod := range sortedSnapshotPods(snapshot) {
		if pod.Object == nil || pod.Object.Namespace != namespace || pod.Object.Name != name {
			continue
		}
		return r.buildPlacementView(pod)
	}
	return nil, nil
}

func (r *PlacementViewREST) snapshot(ctx context.Context) (*index.Snapshot, error) {
	if r.index == nil {
		return &index.Snapshot{}, nil
	}
	return r.index.Snapshot(ctx)
}

func (r *PlacementViewREST) buildPlacementView(pod index.PodSnapshot) (*workspacev1alpha1.PlacementView, error) {
	if pod.Object == nil {
		return nil, nil
	}

	inspection := live.Inspection{}
	if r.resolver != nil {
		var err error
		inspection, err = r.resolver.InspectLiveTargets(live.Request{
			Resource:  corev1.SchemeGroupVersion.WithResource("pods"),
			Namespace: pod.Object.Namespace,
			Name:      pod.Object.Name,
		})
		if err != nil {
			return nil, err
		}
	}

	view := &workspacev1alpha1.PlacementView{
		TypeMeta: metav1.TypeMeta{
			APIVersion: workspacev1alpha1.GroupVersion.String(),
			Kind:       workspacev1alpha1.ResourceKindPlacementView,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: placementViewNameFor(pod.Object.Namespace, pod.Object.Name),
		},
		Status: workspacev1alpha1.PlacementViewStatus{
			SubjectRef: workspacev1alpha1.ObjectReference{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Pod",
				Namespace:  pod.Object.Namespace,
				Name:       pod.Object.Name,
			},
			EligibleClusters:    clustersFromTargets(inspection.Candidates),
			SelectedClusters:    selectedClustersFromInspection(inspection),
			AmbiguousLiveTarget: inspection.Ambiguous,
		},
	}
	return view, nil
}

func splitPlacementViewName(name string) (string, string, bool) {
	namespace, objectName, ok := strings.Cut(name, ".")
	if !ok || namespace == "" || objectName == "" {
		return "", "", false
	}
	return namespace, objectName, true
}

func placementViewNameFor(namespace, name string) string {
	if namespace == "" {
		return name
	}
	return namespace + "." + name
}

func clustersFromTargets(targets []live.Target) []string {
	if len(targets) == 0 {
		return nil
	}
	clusters := make([]string, 0, len(targets))
	seen := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		if _, ok := seen[target.Cluster]; ok {
			continue
		}
		seen[target.Cluster] = struct{}{}
		clusters = append(clusters, target.Cluster)
	}
	sort.Strings(clusters)
	return clusters
}

func selectedClustersFromInspection(inspection live.Inspection) []string {
	if inspection.Ambiguous || len(inspection.Candidates) != 1 {
		return nil
	}
	return []string{inspection.Candidates[0].Cluster}
}

func sortedSnapshotPods(snapshot *index.Snapshot) []index.PodSnapshot {
	if snapshot == nil || len(snapshot.Pods) == 0 {
		return nil
	}
	items := append([]index.PodSnapshot(nil), snapshot.Pods...)
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i].Object
		right := items[j].Object
		if left == nil || right == nil {
			return left != nil
		}
		if left.Namespace != right.Namespace {
			return left.Namespace < right.Namespace
		}
		return left.Name < right.Name
	})
	return items
}
