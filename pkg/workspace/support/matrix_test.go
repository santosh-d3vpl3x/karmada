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
	"sort"
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

var deploymentsGVR = appsv1.SchemeGroupVersion.WithResource("deployments")
var eventsGVR = corev1.SchemeGroupVersion.WithResource("events")
var namespacesGVR = corev1.SchemeGroupVersion.WithResource("namespaces")
var podsGVR = corev1.SchemeGroupVersion.WithResource("pods")
var serviceAccountsGVR = corev1.SchemeGroupVersion.WithResource("serviceaccounts")
var networkPoliciesGVR = networkingv1.SchemeGroupVersion.WithResource("networkpolicies")
var limitRangesGVR = corev1.SchemeGroupVersion.WithResource("limitranges")
var resourceQuotasGVR = corev1.SchemeGroupVersion.WithResource("resourcequotas")
var rolesGVR = rbacv1.SchemeGroupVersion.WithResource("roles")
var roleBindingsGVR = rbacv1.SchemeGroupVersion.WithResource("rolebindings")
var ingressesGVR = networkingv1.SchemeGroupVersion.WithResource("ingresses")
var podDisruptionBudgetsGVR = policyv1.SchemeGroupVersion.WithResource("poddisruptionbudgets")
var horizontalPodAutoscalersGVR = autoscalingv2.SchemeGroupVersion.WithResource("horizontalpodautoscalers")

func TestPhase1Matrix(t *testing.T) {
	if !Supports(deploymentsGVR, "create") {
		t.Fatal("expected deployment create support")
	}
	if Supports(eventsGVR, "create") {
		t.Fatal("did not expect event create support")
	}
}

func TestPodCapabilityIncludesPhase1LiveSubresources(t *testing.T) {
	capability, ok := CapabilityFor(podsGVR)
	if !ok {
		t.Fatal("expected pod capability")
	}
	if capability.Write {
		t.Fatal("did not expect pod write support")
	}
	for _, subresource := range []string{"logs", "exec", "attach", "portforward"} {
		if !SupportsSubresource(podsGVR, subresource) {
			t.Fatalf("expected pod %s support", subresource)
		}
	}
	if SupportsSubresource(podsGVR, "proxy") {
		t.Fatal("did not expect pod proxy support in phase 1")
	}
}

func TestNamespaceCapabilityIsWritable(t *testing.T) {
	capability, ok := CapabilityFor(namespacesGVR)
	if !ok {
		t.Fatal("expected namespace capability")
	}
	if !capability.Read {
		t.Fatal("expected namespace read support")
	}
	if !capability.Watch {
		t.Fatal("expected namespace watch support")
	}
	if !capability.Write {
		t.Fatal("expected namespace write support")
	}
	if !Supports(namespacesGVR, "create") {
		t.Fatal("expected namespace create support")
	}
}

func TestAdditionalPhase2CapabilitiesAreWritable(t *testing.T) {
	tests := []struct {
		name string
		gvr  schema.GroupVersionResource
	}{
		{name: "serviceaccount", gvr: serviceAccountsGVR},
		{name: "networkpolicy", gvr: networkPoliciesGVR},
		{name: "limitrange", gvr: limitRangesGVR},
		{name: "resourcequota", gvr: resourceQuotasGVR},
		{name: "role", gvr: rolesGVR},
		{name: "rolebinding", gvr: roleBindingsGVR},
		{name: "ingress", gvr: ingressesGVR},
		{name: "poddisruptionbudget", gvr: podDisruptionBudgetsGVR},
		{name: "horizontalpodautoscaler", gvr: horizontalPodAutoscalersGVR},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capability, ok := CapabilityFor(tt.gvr)
			if !ok {
				t.Fatalf("expected %s capability", tt.name)
			}
			if !capability.Read {
				t.Fatalf("expected %s read support", tt.name)
			}
			if !capability.Watch {
				t.Fatalf("expected %s watch support", tt.name)
			}
			if !capability.Write {
				t.Fatalf("expected %s write support", tt.name)
			}
			if !Supports(tt.gvr, "create") {
				t.Fatalf("expected %s create support", tt.name)
			}
		})
	}
}

func TestResourcesMatchesCompletedPhase2Baseline(t *testing.T) {
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

	sort.Slice(expected, func(i, j int) bool {
		if expected[i].Group != expected[j].Group {
			return expected[i].Group < expected[j].Group
		}
		if expected[i].Version != expected[j].Version {
			return expected[i].Version < expected[j].Version
		}
		return expected[i].Resource < expected[j].Resource
	})

	if actual := Resources(); !reflect.DeepEqual(actual, expected) {
		t.Fatalf("unexpected phase2 baseline resources: %#v", actual)
	}
}
