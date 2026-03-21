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

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workspacev1alpha1 "github.com/karmada-io/karmada/pkg/apis/workspace/v1alpha1"
	"github.com/karmada-io/karmada/pkg/workspace/support"
)

// CompileDesiredState compiles a workspace mutation into an ordinary Kubernetes source object.
func CompileDesiredState(gvr schema.GroupVersionResource, ws *workspacev1alpha1.Workspace, obj client.Object) (client.Object, error) {
	if ws == nil {
		return nil, fmt.Errorf("workspace is required")
	}
	if obj == nil {
		return nil, fmt.Errorf("source object is required")
	}

	capability, ok := support.CapabilityFor(gvr)
	if !ok || !capability.Write {
		return nil, fmt.Errorf("resource %s is not writable in phase 1", gvr.String())
	}

	out := obj.DeepCopyObject().(client.Object)

	labels := out.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[LabelWorkspaceName] = ws.Name
	out.SetLabels(labels)

	annotations := out.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[AnnotationWorkspaceName] = ws.Name
	annotations[AnnotationWorkspaceResource] = gvr.String()
	if len(ws.UID) > 0 {
		annotations[AnnotationWorkspaceUID] = string(ws.UID)
	}
	out.SetAnnotations(annotations)

	return out, nil
}
