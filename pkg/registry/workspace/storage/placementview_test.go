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

	workspacev1alpha1 "github.com/karmada-io/karmada/pkg/apis/workspace/v1alpha1"
)

func TestPlacementViewRESTMetadata(t *testing.T) {
	rest := NewPlacementViewREST()
	if rest.NamespaceScoped() {
		t.Fatal("expected cluster-scoped placement view storage")
	}
	if rest.GetSingularName() != workspacev1alpha1.ResourceSingularPlacementView {
		t.Fatalf("unexpected singular name: %s", rest.GetSingularName())
	}
	if _, ok := rest.New().(*workspacev1alpha1.PlacementView); !ok {
		t.Fatal("expected placement view object")
	}
}
