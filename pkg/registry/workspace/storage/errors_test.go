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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func TestNewAmbiguousTargetErrorUsesConflict(t *testing.T) {
	err := NewAmbiguousTargetError("pods", "demo")
	if !apierrors.IsConflict(err) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestNewNoEligibleLiveTargetErrorUsesNotFound(t *testing.T) {
	err := NewNoEligibleLiveTargetError("pods", "demo")
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestNewPlacementVisibilityDeniedErrorUsesForbidden(t *testing.T) {
	err := NewPlacementVisibilityDeniedError("placementviews", "demo")
	if !apierrors.IsForbidden(err) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestNewUnsupportedVerbErrorUsesMethodNotSupported(t *testing.T) {
	err := NewUnsupportedVerbError("events", "create")
	if !apierrors.IsMethodNotSupported(err) {
		t.Fatalf("expected method not supported, got %v", err)
	}
}
