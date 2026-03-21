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
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	workspacev1alpha1 "github.com/karmada-io/karmada/pkg/apis/workspace/v1alpha1"
	"github.com/karmada-io/karmada/pkg/workspace/support"
)

// NewUnsupportedResourceError reports a resource that is outside the phase-1 workspace surface.
func NewUnsupportedResourceError(resource string) error {
	return apierrors.NewNotFound(workspacev1alpha1.Resource(resource), resource)
}

// NewUnsupportedVerbError reports a verb that is outside the phase-1 workspace surface.
func NewUnsupportedVerbError(resource, verb string) error {
	return apierrors.NewMethodNotSupported(workspacev1alpha1.Resource(resource), verb)
}

// NewUnsupportedRequestError selects the phase-1 error shape for an unsupported request.
func NewUnsupportedRequestError(resource schema.GroupVersionResource, verb string) error {
	if !support.Exposes(resource) {
		return NewUnsupportedResourceError(resource.Resource)
	}
	return NewUnsupportedVerbError(resource.Resource, verb)
}

// NewAmbiguousTargetError reports that a live request matched more than one eligible target.
func NewAmbiguousTargetError(resource, name string) error {
	return apierrors.NewConflict(workspacev1alpha1.Resource(resource), name, fmt.Errorf("ambiguous live target"))
}

// NewNoEligibleLiveTargetError reports that no live target could be selected.
func NewNoEligibleLiveTargetError(resource, name string) error {
	return apierrors.NewNotFound(workspacev1alpha1.Resource(resource), name)
}

// NewPlacementVisibilityDeniedError reports that placement visibility does not allow the requested view.
func NewPlacementVisibilityDeniedError(resource, name string) error {
	return apierrors.NewForbidden(workspacev1alpha1.Resource(resource), name, fmt.Errorf("placement visibility denied"))
}
