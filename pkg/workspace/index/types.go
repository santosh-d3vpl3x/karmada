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
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// Snapshot is the workspace-indexed runtime view for read-only projected resources.
type Snapshot struct {
	ResourceVersion string
	Pods            []PodSnapshot
	Events          []EventSnapshot
}

// PodSnapshot captures the backing runtime state for one projected pod.
type PodSnapshot struct {
	Object   *corev1.Pod
	Clusters []string
}

// EventSnapshot captures one projected event and the already-resolved involved object reference.
type EventSnapshot struct {
	Object         *corev1.Event
	InvolvedObject corev1.ObjectReference
}

// WatchOptions scopes a runtime watch request against the workspace index.
type WatchOptions struct {
	Namespace       string
	ResourceVersion string
}

// State is the injectable runtime contract consumed by projected storages.
type State interface {
	Snapshot(context.Context) (*Snapshot, error)
	Watch(context.Context, WatchOptions) (watch.Interface, error)
}
