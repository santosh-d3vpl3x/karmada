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

package live

import (
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

type staticState struct {
	targets []Target
	err     error
}

func (s staticState) LiveTargets(Request) ([]Target, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := make([]Target, len(s.targets))
	copy(out, s.targets)
	return out, nil
}

func TestResolveUniqueLiveTarget(t *testing.T) {
	target, err := ResolveLiveTarget(staticState{
		targets: []Target{{Cluster: "member-a"}},
	}, Request{
		Resource:    corev1.SchemeGroupVersion.WithResource("pods"),
		Namespace:   "default",
		Name:        "demo",
		Subresource: "logs",
	})
	if err != nil {
		t.Fatal(err)
	}
	if target.Cluster != "member-a" {
		t.Fatalf("got %q, want %q", target.Cluster, "member-a")
	}
}

func TestResolveLiveTargetReturnsConflictOnAmbiguity(t *testing.T) {
	_, err := ResolveLiveTarget(staticState{
		targets: []Target{
			{Cluster: "member-a"},
			{Cluster: "member-b"},
		},
	}, Request{
		Resource:    corev1.SchemeGroupVersion.WithResource("pods"),
		Namespace:   "default",
		Name:        "demo",
		Subresource: "exec",
	})
	if !errors.Is(err, ErrAmbiguousLiveTarget) {
		t.Fatalf("expected ambiguous target error, got %v", err)
	}
}

func TestResolveLiveTargetReturnsNotFoundWhenNoTargets(t *testing.T) {
	_, err := ResolveLiveTarget(staticState{}, Request{
		Resource:    corev1.SchemeGroupVersion.WithResource("pods"),
		Namespace:   "default",
		Name:        "demo",
		Subresource: "attach",
	})
	if !errors.Is(err, ErrNoLiveTarget) {
		t.Fatalf("expected no-target error, got %v", err)
	}
}

func TestResolveLiveTargetHonorsExplicitClusterSelection(t *testing.T) {
	target, err := ResolveLiveTarget(staticState{
		targets: []Target{{Cluster: "member-a"}, {Cluster: "member-b"}},
	}, Request{
		Resource:    corev1.SchemeGroupVersion.WithResource("pods"),
		Namespace:   "default",
		Name:        "demo",
		Subresource: "logs",
		Selector:    TargetSelector{Cluster: "member-b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if target.Cluster != "member-b" {
		t.Fatalf("got %q, want %q", target.Cluster, "member-b")
	}
}

func TestInspectLiveTargetsReportsCandidatesWithoutSelecting(t *testing.T) {
	inspection, err := InspectLiveTargets(staticState{
		targets: []Target{{Cluster: "member-a"}, {Cluster: "member-b"}},
	}, Request{
		Resource:    corev1.SchemeGroupVersion.WithResource("pods"),
		Namespace:   "default",
		Name:        "demo",
		Subresource: "exec",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !inspection.Ambiguous {
		t.Fatal("expected ambiguous inspection")
	}
	if len(inspection.Candidates) != 2 {
		t.Fatalf("got %d candidates, want 2", len(inspection.Candidates))
	}
}
