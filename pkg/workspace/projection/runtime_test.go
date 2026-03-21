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

package projection

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

func TestProjectedPodNameIsDeterministicOnCollision(t *testing.T) {
	got1 := ProjectPodName("nginx-abc", []string{"member-b", "member-a"})
	got2 := ProjectPodName("nginx-abc", []string{"member-a", "member-b"})

	if got1 != got2 {
		t.Fatalf("expected stable name, got %q and %q", got1, got2)
	}
	if got1 == "nginx-abc" {
		t.Fatal("expected synthesized stable name on collision")
	}
	if errs := validation.IsDNS1123Label(got1); len(errs) != 0 {
		t.Fatalf("expected DNS-label-safe name, got %q: %v", got1, errs)
	}
}

func TestProjectedEventPreservesInvolvedObjectReference(t *testing.T) {
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx-abc.123",
			Namespace: "default",
		},
		InvolvedObject: corev1.ObjectReference{
			Name:      "member-a-nginx-abc",
			Namespace: "default",
			Kind:      "Pod",
		},
	}
	projectedPodRef := corev1.ObjectReference{
		Name:      "nginx-abc-2f4ab6e8",
		Namespace: "default",
		Kind:      "Pod",
	}

	got := ProjectEvent(event, projectedPodRef)
	if got.InvolvedObject.Name != projectedPodRef.Name {
		t.Fatalf("got %s want %s", got.InvolvedObject.Name, projectedPodRef.Name)
	}
	if got.InvolvedObject.Namespace != projectedPodRef.Namespace {
		t.Fatalf("got %s want %s", got.InvolvedObject.Namespace, projectedPodRef.Namespace)
	}
}
