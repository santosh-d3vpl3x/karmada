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
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestDefaultCatalogKeepsPhase2BaselineSurface(t *testing.T) {
	catalog := DefaultCatalog()

	expected := []schema.GroupVersionResource{
		appsv1.SchemeGroupVersion.WithResource("daemonsets"),
		appsv1.SchemeGroupVersion.WithResource("deployments"),
		appsv1.SchemeGroupVersion.WithResource("statefulsets"),
		autoscalingv2.SchemeGroupVersion.WithResource("horizontalpodautoscalers"),
		batchv1.SchemeGroupVersion.WithResource("cronjobs"),
		batchv1.SchemeGroupVersion.WithResource("jobs"),
		corev1.SchemeGroupVersion.WithResource("configmaps"),
		corev1.SchemeGroupVersion.WithResource("events"),
		corev1.SchemeGroupVersion.WithResource("limitranges"),
		corev1.SchemeGroupVersion.WithResource("namespaces"),
		corev1.SchemeGroupVersion.WithResource("pods"),
		corev1.SchemeGroupVersion.WithResource("resourcequotas"),
		corev1.SchemeGroupVersion.WithResource("secrets"),
		corev1.SchemeGroupVersion.WithResource("serviceaccounts"),
		corev1.SchemeGroupVersion.WithResource("services"),
		networkingv1.SchemeGroupVersion.WithResource("ingresses"),
		networkingv1.SchemeGroupVersion.WithResource("networkpolicies"),
		policyv1.SchemeGroupVersion.WithResource("poddisruptionbudgets"),
		rbacv1.SchemeGroupVersion.WithResource("rolebindings"),
		rbacv1.SchemeGroupVersion.WithResource("roles"),
	}

	for _, gvr := range expected {
		entry, ok := catalog.Entry(gvr)
		if !ok {
			t.Fatalf("expected %s", gvr)
		}
		if entry.Resource != gvr {
			t.Fatalf("got %s entry for %s", entry.Resource, gvr)
		}
	}

	if len(catalog.Resources()) != len(expected) {
		t.Fatalf("got %d resources, want %d", len(catalog.Resources()), len(expected))
	}
}

func TestDefaultCatalogPreservesWorkspaceModes(t *testing.T) {
	catalog := DefaultCatalog()

	tests := []struct {
		name     string
		resource schema.GroupVersionResource
		mode     ResourceMode
		write    bool
	}{
		{
			name:     "desired-state deployment",
			resource: appsv1.SchemeGroupVersion.WithResource("deployments"),
			mode:     ResourceModeDesiredState,
			write:    true,
		},
		{
			name:     "logical namespace view",
			resource: corev1.SchemeGroupVersion.WithResource("namespaces"),
			mode:     ResourceModeLogicalClusterView,
			write:    true,
		},
		{
			name:     "projected pod view",
			resource: corev1.SchemeGroupVersion.WithResource("pods"),
			mode:     ResourceModeProjectedRuntime,
			write:    false,
		},
		{
			name:     "projected event view",
			resource: corev1.SchemeGroupVersion.WithResource("events"),
			mode:     ResourceModeProjectedRuntime,
			write:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, ok := catalog.Entry(tt.resource)
			if !ok {
				t.Fatalf("expected %s", tt.resource)
			}
			if entry.Mode != tt.mode {
				t.Fatalf("got mode %q, want %q", entry.Mode, tt.mode)
			}
			if entry.Capability.Write != tt.write {
				t.Fatalf("got write=%t, want %t", entry.Capability.Write, tt.write)
			}
		})
	}
}

func TestDefaultCatalogGroupVersions(t *testing.T) {
	catalog := DefaultCatalog()

	expected := []schema.GroupVersion{
		corev1.SchemeGroupVersion,
		appsv1.SchemeGroupVersion,
		autoscalingv2.SchemeGroupVersion,
		batchv1.SchemeGroupVersion,
		networkingv1.SchemeGroupVersion,
		policyv1.SchemeGroupVersion,
		rbacv1.SchemeGroupVersion,
	}

	if actual := catalog.GroupVersions(); !reflect.DeepEqual(actual, expected) {
		t.Fatalf("unexpected group versions: %#v", actual)
	}
}

func TestModeForReturnsConfiguredResourceMode(t *testing.T) {
	tests := []struct {
		name     string
		resource schema.GroupVersionResource
		mode     ResourceMode
		ok       bool
	}{
		{name: "namespace", resource: corev1.SchemeGroupVersion.WithResource("namespaces"), mode: ResourceModeLogicalClusterView, ok: true},
		{name: "pod", resource: corev1.SchemeGroupVersion.WithResource("pods"), mode: ResourceModeProjectedRuntime, ok: true},
		{name: "configmap", resource: corev1.SchemeGroupVersion.WithResource("configmaps"), mode: ResourceModeDesiredState, ok: true},
		{name: "node", resource: corev1.SchemeGroupVersion.WithResource("nodes"), ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, ok := ModeFor(tt.resource)
			if ok != tt.ok {
				t.Fatalf("got ok=%t, want %t", ok, tt.ok)
			}
			if !tt.ok {
				return
			}
			if mode != tt.mode {
				t.Fatalf("got mode %q, want %q", mode, tt.mode)
			}
		})
	}
}

func TestDefaultCatalogDoesNotAdvertiseDeferredPhase3Resources(t *testing.T) {
	catalog := DefaultCatalog()

	for _, gvr := range []schema.GroupVersionResource{
		corev1.SchemeGroupVersion.WithResource("nodes"),
		rbacv1.SchemeGroupVersion.WithResource("clusterroles"),
		{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"},
		{Group: "example.io", Version: "v1", Resource: "widgets"},
	} {
		if _, ok := catalog.Entry(gvr); ok {
			t.Fatalf("did not expect %s in default catalog", gvr)
		}
	}
}
