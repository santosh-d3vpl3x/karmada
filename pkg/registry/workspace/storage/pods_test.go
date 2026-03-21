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
	"k8s.io/apimachinery/pkg/watch"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"

	"github.com/karmada-io/karmada/pkg/workspace/index"
	"github.com/karmada-io/karmada/pkg/workspace/projection"
)

var _ rest.Getter = (*ProjectedPodREST)(nil)
var _ rest.Lister = (*ProjectedPodREST)(nil)
var _ rest.Watcher = (*ProjectedPodREST)(nil)
var _ rest.Scoper = (*ProjectedPodREST)(nil)
var _ rest.SingularNameProvider = (*ProjectedPodREST)(nil)
var _ rest.TableConvertor = (*ProjectedPodREST)(nil)

func TestProjectedPodStorageListsFromWorkspaceSnapshot(t *testing.T) {
	state := &fakeRuntimeState{
		snapshot: index.Snapshot{
			Pods: []index.PodSnapshot{
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
		},
	}
	storage := NewProjectedPodREST(state)

	ctx := genericapirequest.WithNamespace(context.Background(), "default")
	got, err := storage.List(ctx, &metainternalversion.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}

	list := got.(*corev1.PodList)
	if len(list.Items) != 1 {
		t.Fatalf("got %d pods", len(list.Items))
	}
	want := projection.ProjectPodName("nginx-abc", []string{"member-b", "member-a"})
	if list.Items[0].Name != want {
		t.Fatalf("got pod name %q want %q", list.Items[0].Name, want)
	}
}

func TestProjectedPodStorageResolvesProjectedName(t *testing.T) {
	state := &fakeRuntimeState{
		snapshot: index.Snapshot{
			Pods: []index.PodSnapshot{
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
		},
	}
	storage := NewProjectedPodREST(state)
	ctx := genericapirequest.WithNamespace(context.Background(), "default")
	name := projection.ProjectPodName("nginx-abc", []string{"member-b", "member-a"})
	got, err := storage.Get(ctx, name, &metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pod := got.(*corev1.Pod)
	if pod.Name != name {
		t.Fatalf("got %q want %q", pod.Name, name)
	}
}

func TestProjectedPodStorageDoesNotAdvertiseWrites(t *testing.T) {
	storage := NewProjectedPodREST(&fakeRuntimeState{})

	if _, ok := any(storage).(rest.Creater); ok {
		t.Fatal("pod storage must remain read-only")
	}
	if _, ok := any(storage).(rest.Updater); ok {
		t.Fatal("pod storage must remain read-only")
	}
	if _, ok := any(storage).(rest.GracefulDeleter); ok {
		t.Fatal("pod storage must remain read-only")
	}
}

func TestProjectedPodStorageUsesWatchFromIndex(t *testing.T) {
	state := &fakeRuntimeState{watcher: watch.NewFake()}
	storage := NewProjectedPodREST(state)

	got, err := storage.Watch(context.Background(), &metainternalversion.ListOptions{Watch: true})
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected watch interface")
	}
}

type fakeRuntimeState struct {
	snapshot index.Snapshot
	watcher  watch.Interface
	err      error
}

func (f *fakeRuntimeState) Snapshot(context.Context) (*index.Snapshot, error) {
	if f.err != nil {
		return nil, f.err
	}
	snapshot := f.snapshot
	return &snapshot, nil
}

func (f *fakeRuntimeState) Watch(context.Context, index.WatchOptions) (watch.Interface, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.watcher != nil {
		return f.watcher, nil
	}
	return watch.NewFake(), nil
}
