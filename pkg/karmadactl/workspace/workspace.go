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

package workspace

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/karmada-io/karmada/pkg/karmadactl/util"
)

var (
	workspaceLong = templates.LongDesc(`
		Manage workspace access and client configuration.

		Workspace commands target the tenant-facing workspace API surface rather than the operator-oriented member-cluster proxy.`)

	workspaceExamples = templates.Examples(`
		# Generate a kubeconfig for the team-a workspace
		%[1]s workspace kubeconfig team-a
	`)
)

// NewCmdWorkspace creates the parent workspace command.
func NewCmdWorkspace(f util.Factory, parentCommand string, streams genericiooptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "workspace",
		Short:   "Manage workspace access and configuration",
		Long:    workspaceLong,
		Example: fmt.Sprintf(workspaceExamples, parentCommand),
		Annotations: map[string]string{
			util.TagCommandGroup: util.GroupClusterManagement,
		},
	}

	workspaceParentCommand := fmt.Sprintf("%s workspace", parentCommand)
	cmd.AddCommand(NewCmdKubeconfig(f, workspaceParentCommand, streams))
	return cmd
}
