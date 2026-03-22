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
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
)

// Capability describes the supported workspace behavior for one resource.
type Capability struct {
	Read        bool
	Write       bool
	Watch       bool
	ConnectSubs sets.Set[string]
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

// CapabilityFor returns the supported workspace capability for a resource, if exposed.
func CapabilityFor(resource schema.GroupVersionResource) (Capability, bool) {
	entry, ok := defaultCatalog.Entry(resource)
	if !ok {
		return Capability{}, false
	}
	return entry.Capability, true
}

// Exposes reports whether the resource is part of the supported workspace surface.
func Exposes(resource schema.GroupVersionResource) bool {
	_, ok := defaultCatalog.Entry(resource)
	return ok
}

// Supports reports whether a verb is allowed by the workspace support matrix.
func Supports(resource schema.GroupVersionResource, verb string) bool {
	capability, ok := CapabilityFor(resource)
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

// SupportsSubresource reports whether a live pod subresource is exposed by the workspace surface.
func SupportsSubresource(resource schema.GroupVersionResource, subresource string) bool {
	capability, ok := CapabilityFor(resource)
	if !ok {
		return false
	}
	return capability.ConnectSubs.Has(strings.ToLower(subresource))
}

// Resources returns the full workspace discovery surface in stable order.
func Resources() []schema.GroupVersionResource {
	return defaultCatalog.Resources()
}
