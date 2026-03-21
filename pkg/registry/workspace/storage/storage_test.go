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
	"testing"

	"k8s.io/apiserver/pkg/registry/generic"

	workspacescheme "github.com/karmada-io/karmada/pkg/apis/workspace/scheme"
)

func TestStorageInstallsExpectedResources(t *testing.T) {
	storage, err := NewStorage(workspacescheme.Scheme, generic.RESTOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if storage.Workspaces == nil || storage.PlacementViews == nil {
		t.Fatal("expected workspace root storages")
	}
}
