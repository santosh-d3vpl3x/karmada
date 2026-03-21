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

package index

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRuntimeSnapshotCarriesProjectedRuntimeState(t *testing.T) {
	snapshot := Snapshot{
		ResourceVersion: "17",
		Pods: []PodSnapshot{
			{
				Object: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "nginx-abc",
						Namespace: "default",
					},
				},
				Clusters: []string{"member-b", "member-a"},
			},
		},
		Events: []EventSnapshot{
			{
				Object: &corev1.Event{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "nginx-abc.123",
						Namespace: "default",
					},
				},
				InvolvedObject: corev1.ObjectReference{
					Name:      "nginx-abc",
					Namespace: "default",
					Kind:      "Pod",
				},
			},
		},
	}

	if snapshot.ResourceVersion != "17" {
		t.Fatalf("got resourceVersion %q", snapshot.ResourceVersion)
	}
	if len(snapshot.Pods) != 1 {
		t.Fatalf("got %d pods", len(snapshot.Pods))
	}
	if len(snapshot.Events) != 1 {
		t.Fatalf("got %d events", len(snapshot.Events))
	}
	if snapshot.Events[0].InvolvedObject.Name != "nginx-abc" {
		t.Fatalf("got involved object %q", snapshot.Events[0].InvolvedObject.Name)
	}
}
