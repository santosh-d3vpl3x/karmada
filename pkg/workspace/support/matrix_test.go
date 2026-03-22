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
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

var deploymentsGVR = appsv1.SchemeGroupVersion.WithResource("deployments")
var eventsGVR = corev1.SchemeGroupVersion.WithResource("events")
var namespacesGVR = corev1.SchemeGroupVersion.WithResource("namespaces")
var podsGVR = corev1.SchemeGroupVersion.WithResource("pods")

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

func TestNamespaceCapabilityIsReadOnly(t *testing.T) {
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
	if capability.Write {
		t.Fatal("namespace view must remain read-only")
	}
	if Supports(namespacesGVR, "create") {
		t.Fatal("did not expect namespace create support")
	}
}
