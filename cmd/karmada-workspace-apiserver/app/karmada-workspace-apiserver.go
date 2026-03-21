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

package app

import (
	"context"

	"github.com/spf13/cobra"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"
	"k8s.io/component-base/term"

	workspaceoptions "github.com/karmada-io/karmada/cmd/karmada-workspace-apiserver/app/options"
	"github.com/karmada-io/karmada/pkg/sharedcli"
	"github.com/karmada-io/karmada/pkg/sharedcli/klogflag"
	"github.com/karmada-io/karmada/pkg/version/sharedcommand"
)

const componentName = "karmada-workspace-apiserver"

// NewKarmadaWorkspaceAPIServerCommand creates the workspace apiserver command.
func NewKarmadaWorkspaceAPIServerCommand(ctx context.Context) *cobra.Command {
	logConfig := logsv1.NewLoggingConfiguration()
	fss := cliflag.NamedFlagSets{}

	logsFlagSet := fss.FlagSet("logs")
	logs.AddFlags(logsFlagSet, logs.SkipLoggingConfigurationFlags())
	logsv1.AddFlags(logConfig, logsFlagSet)
	klogflag.Add(logsFlagSet)

	genericFlagSet := fss.FlagSet("generic")
	opts := workspaceoptions.NewOptions()
	opts.AddFlags(genericFlagSet)

	cmd := &cobra.Command{
		Use:  componentName,
		Long: "The karmada-workspace-apiserver starts an aggregated server for the workspace API.",
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := opts.Complete(); err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			return workspaceoptions.Run(ctx, opts)
		},
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			if err := logsv1.ValidateAndApply(logConfig, nil); err != nil {
				return err
			}
			logs.InitLogs()
			return nil
		},
	}

	cmd.AddCommand(sharedcommand.NewCmdVersion(componentName))
	cmd.Flags().AddFlagSet(genericFlagSet)
	cmd.Flags().AddFlagSet(logsFlagSet)

	cols, _, _ := term.TerminalSize(cmd.OutOrStdout())
	sharedcli.SetUsageAndHelpFunc(cmd, fss, cols)
	return cmd
}
