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

package planner

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var podGVR = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}

func TestConservativePushdownAllowsSafeFilters(t *testing.T) {
	plan, err := ConservativePushdown(QueryRequest{
		Resource:      podGVR,
		Namespace:     "team-a",
		LabelSelector: "app=demo",
		FieldSelector: "metadata.name=demo-0,metadata.namespace=team-a",
		Watch:         true,
	}, []string{"member-b", "member-a"})
	if err != nil {
		t.Fatalf("ConservativePushdown() error = %v", err)
	}

	if !plan.Supported() {
		t.Fatalf("expected supported plan, got unsupported=%v", plan.Unsupported)
	}

	if !reflect.DeepEqual(plan.CandidateClusters, []string{"member-a", "member-b"}) {
		t.Fatalf("unexpected candidate clusters: %v", plan.CandidateClusters)
	}

	if plan.Pushdown.LabelSelector != "app=demo" {
		t.Fatalf("unexpected pushdown label selector: %q", plan.Pushdown.LabelSelector)
	}

	if plan.Pushdown.FieldSelector != "metadata.name=demo-0,metadata.namespace=team-a" {
		t.Fatalf("unexpected pushdown field selector: %q", plan.Pushdown.FieldSelector)
	}

	if !plan.Pushdown.Watch {
		t.Fatalf("expected watch to be preserved")
	}
}

func TestConservativePushdownRejectsUnsupportedFieldSelectors(t *testing.T) {
	plan, err := ConservativePushdown(QueryRequest{
		Resource:      podGVR,
		FieldSelector: "status.phase=Running",
	}, []string{"member-a"})
	if err != nil {
		t.Fatalf("ConservativePushdown() error = %v", err)
	}

	if plan.Supported() {
		t.Fatalf("expected unsupported plan")
	}

	if !reflect.DeepEqual(plan.Unsupported, []UnsupportedFeature{UnsupportedFieldSelector}) {
		t.Fatalf("unexpected unsupported features: %v", plan.Unsupported)
	}

	if plan.Pushdown.FieldSelector != "" {
		t.Fatalf("did not expect unsupported field selector to be pushed down")
	}
}

func TestConservativePushdownRejectsUnsupportedOperators(t *testing.T) {
	plan, err := ConservativePushdown(QueryRequest{
		Resource:      podGVR,
		FieldSelector: "metadata.name!=demo-0",
	}, []string{"member-a"})
	if err != nil {
		t.Fatalf("ConservativePushdown() error = %v", err)
	}

	if plan.Supported() {
		t.Fatalf("expected unsupported plan")
	}
}

func TestConservativePushdownRejectsInvalidSelectors(t *testing.T) {
	if _, err := ConservativePushdown(QueryRequest{Resource: podGVR, LabelSelector: "app in ("}, nil); err == nil {
		t.Fatal("expected invalid label selector error")
	}

	if _, err := ConservativePushdown(QueryRequest{Resource: podGVR, FieldSelector: "metadata.name=="}, nil); err == nil {
		t.Fatal("expected invalid field selector error")
	}
}

func TestConservativePushdownRejectsUnsupportedResources(t *testing.T) {
	plan, err := ConservativePushdown(QueryRequest{
		Resource: corev1.SchemeGroupVersion.WithResource("nodes"),
	}, []string{"member-a"})
	if err != nil {
		t.Fatalf("ConservativePushdown() error = %v", err)
	}

	if plan.Supported() {
		t.Fatalf("expected unsupported plan")
	}

	if !reflect.DeepEqual(plan.Unsupported, []UnsupportedFeature{UnsupportedResource}) {
		t.Fatalf("unexpected unsupported features: %v", plan.Unsupported)
	}
}
