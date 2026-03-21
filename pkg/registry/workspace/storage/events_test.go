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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"

	"github.com/karmada-io/karmada/pkg/workspace/index"
)

var _ rest.Getter = (*ProjectedEventREST)(nil)
var _ rest.Lister = (*ProjectedEventREST)(nil)
var _ rest.Watcher = (*ProjectedEventREST)(nil)
var _ rest.Scoper = (*ProjectedEventREST)(nil)
var _ rest.SingularNameProvider = (*ProjectedEventREST)(nil)
var _ rest.TableConvertor = (*ProjectedEventREST)(nil)

func TestProjectedEventStoragePreservesInvolvedObjectReference(t *testing.T) {
	projectedPodRef := corev1.ObjectReference{
		Name:      "nginx-abc-2f4ab6e8",
		Namespace: "default",
		Kind:      "Pod",
	}
	state := &fakeRuntimeState{
		snapshot: index.Snapshot{
			Events: []index.EventSnapshot{
				{
					Object: &corev1.Event{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "nginx-abc.123",
							Namespace: "default",
						},
					},
					InvolvedObject: projectedPodRef,
				},
			},
		},
	}
	storage := NewProjectedEventREST(state)

	ctx := genericapirequest.WithNamespace(context.Background(), "default")
	got, err := storage.Get(ctx, "nginx-abc.123", &metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	event := got.(*corev1.Event)
	if event.InvolvedObject.Name != projectedPodRef.Name {
		t.Fatalf("got %q want %q", event.InvolvedObject.Name, projectedPodRef.Name)
	}
	if event.InvolvedObject.Namespace != projectedPodRef.Namespace {
		t.Fatalf("got %q want %q", event.InvolvedObject.Namespace, projectedPodRef.Namespace)
	}
}

func TestProjectedEventStorageListsFromWorkspaceSnapshot(t *testing.T) {
	state := &fakeRuntimeState{
		snapshot: index.Snapshot{
			Events: []index.EventSnapshot{
				{
					Object: &corev1.Event{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "nginx-abc.123",
							Namespace: "default",
						},
					},
					InvolvedObject: corev1.ObjectReference{
						Name:      "nginx-abc-2f4ab6e8",
						Namespace: "default",
						Kind:      "Pod",
					},
				},
			},
		},
	}
	storage := NewProjectedEventREST(state)

	ctx := genericapirequest.WithNamespace(context.Background(), "default")
	got, err := storage.List(ctx, &metainternalversion.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	list := got.(*corev1.EventList)
	if len(list.Items) != 1 {
		t.Fatalf("got %d events", len(list.Items))
	}
	if list.Items[0].InvolvedObject.Name != "nginx-abc-2f4ab6e8" {
		t.Fatalf("got %q", list.Items[0].InvolvedObject.Name)
	}
}

func TestProjectedEventStorageDoesNotAdvertiseWrites(t *testing.T) {
	storage := NewProjectedEventREST(&fakeRuntimeState{})

	if _, ok := any(storage).(rest.Creater); ok {
		t.Fatal("event storage must remain read-only")
	}
	if _, ok := any(storage).(rest.Updater); ok {
		t.Fatal("event storage must remain read-only")
	}
	if _, ok := any(storage).(rest.GracefulDeleter); ok {
		t.Fatal("event storage must remain read-only")
	}
}
