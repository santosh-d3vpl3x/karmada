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

package support

import (
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ResourceMode string

const (
	ResourceModeDesiredState       ResourceMode = "DesiredState"
	ResourceModeProjectedRuntime   ResourceMode = "ProjectedRuntime"
	ResourceModeLogicalClusterView ResourceMode = "LogicalClusterView"
)

type ResourceRoute string

const (
	ResourceRouteDesiredState     ResourceRoute = "DesiredState"
	ResourceRouteLogicalNamespace ResourceRoute = "LogicalNamespace"
	ResourceRouteProjectedPod     ResourceRoute = "ProjectedPod"
	ResourceRouteProjectedEvent   ResourceRoute = "ProjectedEvent"
)

type CatalogEntry struct {
	Resource   schema.GroupVersionResource
	Capability Capability
	Mode       ResourceMode
	Route      ResourceRoute
}

type Catalog struct {
	entries map[schema.GroupVersionResource]CatalogEntry
}

var defaultCatalog = newCatalog([]CatalogEntry{
	{Resource: corev1.SchemeGroupVersion.WithResource("namespaces"), Capability: newCapability(true, true, true), Mode: ResourceModeLogicalClusterView, Route: ResourceRouteLogicalNamespace},
	{Resource: corev1.SchemeGroupVersion.WithResource("configmaps"), Capability: newCapability(true, true, true), Mode: ResourceModeDesiredState, Route: ResourceRouteDesiredState},
	{Resource: corev1.SchemeGroupVersion.WithResource("secrets"), Capability: newCapability(true, true, true), Mode: ResourceModeDesiredState, Route: ResourceRouteDesiredState},
	{Resource: corev1.SchemeGroupVersion.WithResource("serviceaccounts"), Capability: newCapability(true, true, true), Mode: ResourceModeDesiredState, Route: ResourceRouteDesiredState},
	{Resource: corev1.SchemeGroupVersion.WithResource("limitranges"), Capability: newCapability(true, true, true), Mode: ResourceModeDesiredState, Route: ResourceRouteDesiredState},
	{Resource: corev1.SchemeGroupVersion.WithResource("resourcequotas"), Capability: newCapability(true, true, true), Mode: ResourceModeDesiredState, Route: ResourceRouteDesiredState},
	{Resource: rbacv1.SchemeGroupVersion.WithResource("roles"), Capability: newCapability(true, true, true), Mode: ResourceModeDesiredState, Route: ResourceRouteDesiredState},
	{Resource: rbacv1.SchemeGroupVersion.WithResource("rolebindings"), Capability: newCapability(true, true, true), Mode: ResourceModeDesiredState, Route: ResourceRouteDesiredState},
	{Resource: networkingv1.SchemeGroupVersion.WithResource("networkpolicies"), Capability: newCapability(true, true, true), Mode: ResourceModeDesiredState, Route: ResourceRouteDesiredState},
	{Resource: networkingv1.SchemeGroupVersion.WithResource("ingresses"), Capability: newCapability(true, true, true), Mode: ResourceModeDesiredState, Route: ResourceRouteDesiredState},
	{Resource: policyv1.SchemeGroupVersion.WithResource("poddisruptionbudgets"), Capability: newCapability(true, true, true), Mode: ResourceModeDesiredState, Route: ResourceRouteDesiredState},
	{Resource: autoscalingv2.SchemeGroupVersion.WithResource("horizontalpodautoscalers"), Capability: newCapability(true, true, true), Mode: ResourceModeDesiredState, Route: ResourceRouteDesiredState},
	{Resource: corev1.SchemeGroupVersion.WithResource("services"), Capability: newCapability(true, true, true), Mode: ResourceModeDesiredState, Route: ResourceRouteDesiredState},
	{Resource: appsv1.SchemeGroupVersion.WithResource("deployments"), Capability: newCapability(true, true, true), Mode: ResourceModeDesiredState, Route: ResourceRouteDesiredState},
	{Resource: appsv1.SchemeGroupVersion.WithResource("statefulsets"), Capability: newCapability(true, true, true), Mode: ResourceModeDesiredState, Route: ResourceRouteDesiredState},
	{Resource: appsv1.SchemeGroupVersion.WithResource("daemonsets"), Capability: newCapability(true, true, true), Mode: ResourceModeDesiredState, Route: ResourceRouteDesiredState},
	{Resource: batchv1.SchemeGroupVersion.WithResource("jobs"), Capability: newCapability(true, true, true), Mode: ResourceModeDesiredState, Route: ResourceRouteDesiredState},
	{Resource: batchv1.SchemeGroupVersion.WithResource("cronjobs"), Capability: newCapability(true, true, true), Mode: ResourceModeDesiredState, Route: ResourceRouteDesiredState},
	{Resource: corev1.SchemeGroupVersion.WithResource("pods"), Capability: newCapability(true, false, true, "logs", "exec", "attach", "portforward"), Mode: ResourceModeProjectedRuntime, Route: ResourceRouteProjectedPod},
	{Resource: corev1.SchemeGroupVersion.WithResource("events"), Capability: newCapability(true, false, true), Mode: ResourceModeProjectedRuntime, Route: ResourceRouteProjectedEvent},
})

func DefaultCatalog() Catalog {
	return cloneCatalog(defaultCatalog)
}

func (c Catalog) Entry(resource schema.GroupVersionResource) (CatalogEntry, bool) {
	entry, ok := c.entries[resource]
	if !ok {
		return CatalogEntry{}, false
	}
	return cloneCatalogEntry(entry), true
}

func (c Catalog) Resources() []schema.GroupVersionResource {
	resources := make([]schema.GroupVersionResource, 0, len(c.entries))
	for resource := range c.entries {
		resources = append(resources, resource)
	}
	sort.Slice(resources, func(i, j int) bool {
		if resources[i].Group != resources[j].Group {
			return resources[i].Group < resources[j].Group
		}
		if resources[i].Version != resources[j].Version {
			return resources[i].Version < resources[j].Version
		}
		return resources[i].Resource < resources[j].Resource
	})
	return resources
}

func (c Catalog) GroupVersions() []schema.GroupVersion {
	seen := make(map[schema.GroupVersion]struct{}, len(c.entries))
	groupVersions := make([]schema.GroupVersion, 0, len(c.entries))
	for resource := range c.entries {
		groupVersion := resource.GroupVersion()
		if _, ok := seen[groupVersion]; ok {
			continue
		}
		seen[groupVersion] = struct{}{}
		groupVersions = append(groupVersions, groupVersion)
	}
	sort.Slice(groupVersions, func(i, j int) bool {
		if groupVersions[i].Group != groupVersions[j].Group {
			return groupVersions[i].Group < groupVersions[j].Group
		}
		return groupVersions[i].Version < groupVersions[j].Version
	})
	return groupVersions
}

func GroupVersions() []schema.GroupVersion {
	return defaultCatalog.GroupVersions()
}

func ModeFor(resource schema.GroupVersionResource) (ResourceMode, bool) {
	entry, ok := defaultCatalog.Entry(resource)
	if !ok {
		return "", false
	}
	return entry.Mode, true
}

func RouteFor(resource schema.GroupVersionResource) (ResourceRoute, bool) {
	entry, ok := defaultCatalog.Entry(resource)
	if !ok {
		return "", false
	}
	return entry.Route, true
}

func newCatalog(entries []CatalogEntry) Catalog {
	catalog := Catalog{entries: make(map[schema.GroupVersionResource]CatalogEntry, len(entries))}
	for _, entry := range entries {
		catalog.entries[entry.Resource] = cloneCatalogEntry(entry)
	}
	return catalog
}

func cloneCatalog(c Catalog) Catalog {
	clone := Catalog{entries: make(map[schema.GroupVersionResource]CatalogEntry, len(c.entries))}
	for resource, entry := range c.entries {
		clone.entries[resource] = cloneCatalogEntry(entry)
	}
	return clone
}

func cloneCatalogEntry(entry CatalogEntry) CatalogEntry {
	entry.Capability = cloneCapability(entry.Capability)
	return entry
}
