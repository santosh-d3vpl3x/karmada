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
	"context"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/kubectl/pkg/util/templates"

	workspacev1alpha1 "github.com/karmada-io/karmada/pkg/apis/workspace/v1alpha1"
	workspaceclientv1alpha1 "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/typed/workspace/v1alpha1"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
	utilcomp "github.com/karmada-io/karmada/pkg/karmadactl/util/completion"
)

var (
	workspaceKubeconfigLong = templates.LongDesc(`
		Generate a kubeconfig for a workspace endpoint.

		The command reuses the auth configuration from the selected Karmada context and points clients at the published workspace URL. If the workspace has not published a URL yet, the phase-1 proxy path shape is used as a bootstrap fallback.`)

	workspaceKubeconfigExample = templates.Examples(`
		# Print a kubeconfig for the team-a workspace
		%[1]s kubeconfig team-a

		# Override the generated context name
		%[1]s kubeconfig team-a --context-name=team-a-workspace
	`)
)

type CommandKubeconfigOptions struct {
	ContextName string
	Flatten     bool
}

// NewCmdKubeconfig returns a command that generates a workspace-scoped kubeconfig.
func NewCmdKubeconfig(f util.Factory, parentCommand string, streams genericiooptions.IOStreams) *cobra.Command {
	o := &CommandKubeconfigOptions{Flatten: true}

	cmd := &cobra.Command{
		Use:     "kubeconfig WORKSPACE",
		Short:   "Generate a kubeconfig for a workspace endpoint",
		Long:    workspaceKubeconfigLong,
		Args:    cobra.ExactArgs(1),
		Example: fmt.Sprintf(workspaceKubeconfigExample, parentCommand),
		RunE: func(_ *cobra.Command, args []string) error {
			return o.Run(f, args[0], streams.Out)
		},
		Annotations: map[string]string{
			util.TagCommandGroup: util.GroupClusterManagement,
		},
	}

	flags := cmd.Flags()
	options.AddKubeConfigFlags(flags)
	flags.StringVar(&o.ContextName, "context-name", "", "Override the generated kubeconfig context name.")
	flags.BoolVar(&o.Flatten, "flatten", o.Flatten, "Embed referenced CA and client certificate/key files into the generated kubeconfig.")

	utilcomp.RegisterCompletionFuncForKarmadaContextFlag(cmd)
	return cmd
}

// Run writes the generated kubeconfig to the provided writer.
func (o *CommandKubeconfigOptions) Run(f util.Factory, workspaceName string, out io.Writer) error {
	rawConfig, err := f.ToRawKubeConfigLoader().RawConfig()
	if err != nil {
		return err
	}

	selectedContext := rawConfig.CurrentContext
	if override := *options.DefaultConfigFlags.Context; override != "" {
		selectedContext = override
	}

	client, err := f.KarmadaClientSet()
	if err != nil {
		return err
	}
	workspaceURL, err := resolveWorkspaceURL(context.TODO(), client.WorkspaceV1alpha1().Workspaces(), workspaceName)
	if err != nil {
		return err
	}

	kubeconfig, err := buildWorkspaceKubeconfigForContext(&rawConfig, selectedContext, workspaceName, workspaceURL, o.ContextName, o.Flatten)
	if err != nil {
		return err
	}

	data, err := clientcmd.Write(*kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to serialize kubeconfig: %w", err)
	}

	_, err = out.Write(data)
	return err
}

func resolveWorkspaceURL(ctx context.Context, workspaces workspaceclientv1alpha1.WorkspaceInterface, workspaceName string) (string, error) {
	workspace, err := workspaces.Get(ctx, workspaceName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsMethodNotSupported(err) {
			return "", nil
		}
		return "", err
	}

	return workspace.Status.URL, nil
}

func buildWorkspaceKubeconfig(config *clientcmdapi.Config, workspaceName, workspaceURL string) (*clientcmdapi.Config, error) {
	return buildWorkspaceKubeconfigForContext(config, config.CurrentContext, workspaceName, workspaceURL, "", false)
}

func buildWorkspaceKubeconfigForContext(config *clientcmdapi.Config, karmadaContextName, workspaceName, workspaceURL, contextName string, flatten bool) (*clientcmdapi.Config, error) {
	if karmadaContextName == "" {
		return nil, fmt.Errorf("no karmada context selected")
	}

	minifiedConfig := config.DeepCopy()
	minifiedConfig.CurrentContext = karmadaContextName
	if err := clientcmdapi.MinifyConfig(minifiedConfig); err != nil {
		return nil, err
	}
	if flatten {
		if err := clientcmdapi.FlattenConfig(minifiedConfig); err != nil {
			return nil, err
		}
	}

	sourceContext, ok := minifiedConfig.Contexts[karmadaContextName]
	if !ok {
		return nil, fmt.Errorf("cannot locate context %q", karmadaContextName)
	}
	if sourceContext.Cluster == "" {
		return nil, fmt.Errorf("selected context %q does not define a cluster", karmadaContextName)
	}
	if sourceContext.AuthInfo == "" {
		return nil, fmt.Errorf("selected context %q does not define a user", karmadaContextName)
	}

	sourceCluster, ok := minifiedConfig.Clusters[sourceContext.Cluster]
	if !ok {
		return nil, fmt.Errorf("cannot locate cluster %q", sourceContext.Cluster)
	}
	sourceAuthInfo, ok := minifiedConfig.AuthInfos[sourceContext.AuthInfo]
	if !ok {
		return nil, fmt.Errorf("cannot locate user %q", sourceContext.AuthInfo)
	}

	if contextName == "" {
		contextName = workspaceContextName(workspaceName)
	}
	clusterName := contextName
	serverURL := workspaceURL
	if serverURL == "" {
		serverURL = workspaceProxyURL(sourceCluster.Server, workspaceName)
	}

	generatedConfig := clientcmdapi.NewConfig()
	generatedConfig.APIVersion = "v1"
	generatedConfig.Kind = "Config"
	generatedConfig.AuthInfos[sourceContext.AuthInfo] = sourceAuthInfo.DeepCopy()
	generatedConfig.Clusters[clusterName] = sourceCluster.DeepCopy()
	generatedConfig.Clusters[clusterName].Server = serverURL
	generatedConfig.Contexts[contextName] = sourceContext.DeepCopy()
	generatedConfig.Contexts[contextName].Cluster = clusterName
	generatedConfig.CurrentContext = contextName

	return generatedConfig, nil
}

func workspaceContextName(workspaceName string) string {
	return fmt.Sprintf("workspace-%s", workspaceName)
}

func workspaceProxyURL(karmadaAPIServer, workspaceName string) string {
	u, err := url.Parse(karmadaAPIServer)
	if err != nil {
		return karmadaAPIServer + fmt.Sprintf("/apis/%s/%s/workspaces/%s/proxy/", workspacev1alpha1.GroupName, workspacev1alpha1.GroupVersion.Version, workspaceName)
	}

	u.Path = path.Join(u.Path, "apis", workspacev1alpha1.GroupName, workspacev1alpha1.GroupVersion.Version, "workspaces", workspaceName, "proxy")
	if !strings.HasPrefix(u.Path, "/") {
		u.Path = "/" + u.Path
	}
	if !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
	}
	return u.String()
}
