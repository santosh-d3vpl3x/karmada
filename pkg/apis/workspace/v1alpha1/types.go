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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

const (
	ResourceKindWorkspace            = "Workspace"
	ResourceSingularWorkspace        = "workspace"
	ResourcePluralWorkspace          = "workspaces"
	ResourceNamespaceScopedWorkspace = false

	ResourceKindPlacementView            = "PlacementView"
	ResourceSingularPlacementView        = "placementview"
	ResourcePluralPlacementView          = "placementviews"
	ResourceNamespaceScopedPlacementView = false
)

type NamespacePolicyMode string

const (
	NamespacePolicyModeShared    NamespacePolicyMode = "Shared"
	NamespacePolicyModePrefixed  NamespacePolicyMode = "Prefixed"
	NamespacePolicyModeDedicated NamespacePolicyMode = "Dedicated"
)

type PlacementVisibility string

const (
	PlacementVisibilityHidden  PlacementVisibility = "Hidden"
	PlacementVisibilitySummary PlacementVisibility = "Summary"
	PlacementVisibilityDebug   PlacementVisibility = "Debug"
)

type LiveSubresource string

const (
	LiveSubresourceLogs        LiveSubresource = "logs"
	LiveSubresourceExec        LiveSubresource = "exec"
	LiveSubresourceAttach      LiveSubresource = "attach"
	LiveSubresourcePortForward LiveSubresource = "portforward"
	LiveSubresourceProxy       LiveSubresource = "proxy"
)

// ObjectReference identifies a logical workspace subject.
type ObjectReference struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
	Name       string `json:"name,omitempty"`
}

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:path=workspaces,scope="Cluster"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Workspace defines the tenant-facing logical cluster boundary for projected access.
type Workspace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkspaceSpec   `json:"spec,omitempty"`
	Status WorkspaceStatus `json:"status,omitempty"`
}

// WorkspaceSpec describes the desired workspace behavior.
type WorkspaceSpec struct {
	// ClusterSelector identifies the member clusters that may back this workspace.
	ClusterSelector policyv1alpha1.ClusterAffinity `json:"clusterSelector"`

	// NamespacePolicy defines how namespaces are exposed inside the workspace.
	NamespacePolicy NamespacePolicy `json:"namespacePolicy,omitempty"`

	// PlacementVisibility controls how placement details are shown.
	PlacementVisibility PlacementVisibility `json:"placementVisibility,omitempty"`

	// LiveAccess enables live subresources such as logs and exec.
	LiveAccess *LiveAccessPolicy `json:"liveAccess,omitempty"`
}

// NamespacePolicy defines how namespaces are exposed in a workspace.
type NamespacePolicy struct {
	Mode NamespacePolicyMode `json:"mode,omitempty"`

	// Namespaces optionally bounds the visible namespace set for the workspace.
	Namespaces []string `json:"namespaces,omitempty"`
}

// LiveAccessPolicy describes which live subresources are enabled for a workspace.
type LiveAccessPolicy struct {
	Enabled bool `json:"enabled,omitempty"`

	// Subresources lists the live operations allowed through the workspace API.
	Subresources []LiveSubresource `json:"subresources,omitempty"`
}

// WorkspaceStatus reports the observed workspace state.
type WorkspaceStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// URL is the workspace endpoint used to generate kubeconfigs.
	URL string `json:"url,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkspaceList is a collection of Workspace resources.
type WorkspaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Workspace `json:"items"`
}

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:path=placementviews,scope="Cluster"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PlacementView is a read-only debug surface for workspace placement and live-target state.
type PlacementView struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status PlacementViewStatus `json:"status,omitempty"`
}

// PlacementViewStatus reports placement and realization details for a logical workspace object.
type PlacementViewStatus struct {
	SubjectRef          ObjectReference    `json:"subjectRef,omitempty"`
	EligibleClusters    []string           `json:"eligibleClusters,omitempty"`
	SelectedClusters    []string           `json:"selectedClusters,omitempty"`
	AmbiguousLiveTarget bool               `json:"ambiguousLiveTarget,omitempty"`
	Conditions          []metav1.Condition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PlacementViewList is a collection of PlacementView resources.
type PlacementViewList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PlacementView `json:"items"`
}
