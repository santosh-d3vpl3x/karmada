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

package facade

const (
	// LabelWorkspaceName identifies the logical workspace that owns a compiled source object.
	LabelWorkspaceName = "workspace.karmada.io/name"

	// AnnotationWorkspaceName identifies the logical workspace that owns a compiled source object.
	AnnotationWorkspaceName = "workspace.karmada.io/name"

	// AnnotationWorkspaceResource records the source GVR used to compile the object.
	AnnotationWorkspaceResource = "workspace.karmada.io/resource"

	// AnnotationWorkspaceUID records the workspace UID when one is available.
	AnnotationWorkspaceUID = "workspace.karmada.io/uid"
)
