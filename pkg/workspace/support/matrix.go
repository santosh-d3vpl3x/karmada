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
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
)

// Capability describes the phase-1 workspace behavior for one resource.
type Capability struct {
	Read        bool
	Write       bool
	Watch       bool
	ConnectSubs sets.Set[string]
}

var phase1Matrix = map[schema.GroupVersionResource]Capability{
	corev1.SchemeGroupVersion.WithResource("namespaces"):   newCapability(true, false, true),
	corev1.SchemeGroupVersion.WithResource("configmaps"):   newCapability(true, true, true),
	corev1.SchemeGroupVersion.WithResource("secrets"):      newCapability(true, true, true),
	corev1.SchemeGroupVersion.WithResource("services"):     newCapability(true, true, true),
	appsv1.SchemeGroupVersion.WithResource("deployments"):  newCapability(true, true, true),
	appsv1.SchemeGroupVersion.WithResource("statefulsets"): newCapability(true, true, true),
	appsv1.SchemeGroupVersion.WithResource("daemonsets"):   newCapability(true, true, true),
	batchv1.SchemeGroupVersion.WithResource("jobs"):        newCapability(true, true, true),
	batchv1.SchemeGroupVersion.WithResource("cronjobs"):    newCapability(true, true, true),
	corev1.SchemeGroupVersion.WithResource("pods"):         newCapability(true, false, true, "logs", "exec", "attach", "portforward"),
	corev1.SchemeGroupVersion.WithResource("events"):       newCapability(true, false, true),
}

func newCapability(read, write, watch bool, connectSubs ...string) Capability {
	return Capability{
		Read:        read,
		Write:       write,
		Watch:       watch,
		ConnectSubs: sets.New[string](connectSubs...),
	}
}

func cloneCapability(capability Capability) Capability {
	clone := Capability{
		Read:        capability.Read,
		Write:       capability.Write,
		Watch:       capability.Watch,
		ConnectSubs: sets.New[string](),
	}
	for subresource := range capability.ConnectSubs {
		clone.ConnectSubs.Insert(subresource)
	}
	return clone
}

// CapabilityFor returns the phase-1 capability for a resource, if exposed.
func CapabilityFor(resource schema.GroupVersionResource) (Capability, bool) {
	capability, ok := phase1Matrix[resource]
	if !ok {
		return Capability{}, false
	}
	return cloneCapability(capability), true
}

// Exposes reports whether the resource is part of the phase-1 workspace surface.
func Exposes(resource schema.GroupVersionResource) bool {
	_, ok := phase1Matrix[resource]
	return ok
}

// Supports reports whether a verb is allowed by the phase-1 matrix.
func Supports(resource schema.GroupVersionResource, verb string) bool {
	capability, ok := phase1Matrix[resource]
	if !ok {
		return false
	}

	switch strings.ToLower(verb) {
	case "get", "list":
		return capability.Read
	case "watch":
		return capability.Watch
	case "create", "update", "patch", "delete", "deletecollection":
		return capability.Write
	default:
		return false
	}
}

// SupportsSubresource reports whether a live pod subresource is exposed in phase 1.
func SupportsSubresource(resource schema.GroupVersionResource, subresource string) bool {
	capability, ok := phase1Matrix[resource]
	if !ok {
		return false
	}
	return capability.ConnectSubs.Has(strings.ToLower(subresource))
}

// Resources returns the full phase-1 discovery surface in stable order.
func Resources() []schema.GroupVersionResource {
	resources := make([]schema.GroupVersionResource, 0, len(phase1Matrix))
	for resource := range phase1Matrix {
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
