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
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	workspacev1alpha1 "github.com/karmada-io/karmada/pkg/apis/workspace/v1alpha1"
)

func TestCompileDeploymentWriteToSourceObject(t *testing.T) {
	workspace := &workspacev1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{Name: "team-a"},
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx",
			Namespace: "team-a",
		},
	}

	obj, err := CompileDesiredState(appsv1.SchemeGroupVersion.WithResource("deployments"), workspace, deployment)
	if err != nil {
		t.Fatal(err)
	}

	if got := obj.GetNamespace(); got != "team-a" {
		t.Fatalf("got namespace %q", got)
	}
	if got := obj.GetAnnotations()[AnnotationWorkspaceName]; got != "team-a" {
		t.Fatalf("got workspace annotation %q", got)
	}
	if got := obj.GetAnnotations()[AnnotationWorkspaceResource]; got != schema.GroupVersionResource(appsv1.SchemeGroupVersion.WithResource("deployments")).String() {
		t.Fatalf("got workspace resource annotation %q", got)
	}
}
