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

package options

import (
	"context"
	"fmt"
	"net"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/endpoints/openapi"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/storage/storagebackend"
	"k8s.io/apiserver/pkg/util/compatibility"
	"k8s.io/client-go/kubernetes"
	netutils "k8s.io/utils/net"

	workspacescheme "github.com/karmada-io/karmada/pkg/apis/workspace/scheme"
	workspacev1alpha1 "github.com/karmada-io/karmada/pkg/apis/workspace/v1alpha1"
	generatedopenapi "github.com/karmada-io/karmada/pkg/generated/openapi"
	workspaceapiserver "github.com/karmada-io/karmada/pkg/workspace"
)

const defaultEtcdPathPrefix = "/registry"

// Options contains command line parameters for the workspace apiserver.
type Options struct {
	Etcd             *genericoptions.EtcdOptions
	SecureServing    *genericoptions.SecureServingOptionsWithLoopback
	Authentication   *genericoptions.DelegatingAuthenticationOptions
	Authorization    *genericoptions.DelegatingAuthorizationOptions
	Audit            *genericoptions.AuditOptions
	Features         *genericoptions.FeatureOptions
	CoreAPI          *genericoptions.CoreAPIOptions
	ServerRunOptions *genericoptions.ServerRunOptions
}

// NewOptions returns a new Options.
func NewOptions() *Options {
	o := &Options{
		Etcd:             genericoptions.NewEtcdOptions(storagebackend.NewDefaultConfig(defaultEtcdPathPrefix, workspacescheme.Codecs.LegacyCodec(schema.GroupVersion{Group: workspacev1alpha1.GroupVersion.Group, Version: workspacev1alpha1.GroupVersion.Version}))),
		SecureServing:    genericoptions.NewSecureServingOptions().WithLoopback(),
		Authentication:   genericoptions.NewDelegatingAuthenticationOptions(),
		Authorization:    genericoptions.NewDelegatingAuthorizationOptions(),
		Audit:            genericoptions.NewAuditOptions(),
		Features:         genericoptions.NewFeatureOptions(),
		CoreAPI:          genericoptions.NewCoreAPIOptions(),
		ServerRunOptions: genericoptions.NewServerRunOptions(),
	}
	o.Etcd.StorageConfig.EncodeVersioner = runtime.NewMultiGroupVersioner(schema.GroupVersion{Group: workspacev1alpha1.GroupVersion.Group, Version: workspacev1alpha1.GroupVersion.Version}, schema.GroupKind{Group: workspacev1alpha1.GroupName})
	return o
}

// AddFlags adds flags to the specified FlagSet.
func (o *Options) AddFlags(flags *pflag.FlagSet) {
	o.Etcd.AddFlags(flags)
	o.SecureServing.AddFlags(flags)
	o.Authentication.AddFlags(flags)
	o.Authorization.AddFlags(flags)
	o.Audit.AddFlags(flags)
	o.Features.AddFlags(flags)
	o.CoreAPI.AddFlags(flags)
	o.ServerRunOptions.AddUniversalFlags(flags)

	flags.Lookup("kubeconfig").Usage = "Path to karmada control plane kubeconfig file."
}

// ApplyTo adds Options to the server configuration.
func (o *Options) ApplyTo(config *genericapiserver.RecommendedConfig) error {
	if err := o.Etcd.ApplyTo(&config.Config); err != nil {
		return err
	}
	if err := o.SecureServing.ApplyTo(&config.Config.SecureServing, &config.Config.LoopbackClientConfig); err != nil {
		return err
	}
	if err := o.Authentication.ApplyTo(&config.Config.Authentication, config.SecureServing, config.OpenAPIConfig); err != nil {
		return err
	}
	if err := o.Authorization.ApplyTo(&config.Config.Authorization); err != nil {
		return err
	}
	if err := o.Audit.ApplyTo(&config.Config); err != nil {
		return err
	}
	if err := o.CoreAPI.ApplyTo(config); err != nil {
		return err
	}
	if err := o.ServerRunOptions.ApplyTo(&config.Config); err != nil {
		return err
	}
	kubeClient, err := kubernetes.NewForConfig(config.ClientConfig)
	if err != nil {
		return err
	}
	if err = o.Features.ApplyTo(&config.Config, kubeClient, config.SharedInformerFactory); err != nil {
		return err
	}
	return nil
}

// Complete fills in fields required to have valid data.
func (o *Options) Complete() error {
	return o.ServerRunOptions.Complete()
}

// Validate validates Options.
func (o *Options) Validate() error {
	var errs []error
	errs = append(errs, o.Etcd.Validate()...)
	errs = append(errs, o.SecureServing.Validate()...)
	errs = append(errs, o.Authentication.Validate()...)
	errs = append(errs, o.Authorization.Validate()...)
	errs = append(errs, o.Audit.Validate()...)
	errs = append(errs, o.Features.Validate()...)
	errs = append(errs, o.CoreAPI.Validate()...)
	errs = append(errs, o.ServerRunOptions.Validate()...)
	return utilerrors.NewAggregate(errs)
}

// Run runs the workspace apiserver with options.
func Run(ctx context.Context, o *Options) error {
	config, err := configForOptions(o)
	if err != nil {
		return err
	}
	server, err := config.Complete().New()
	if err != nil {
		return err
	}
	return server.GenericAPIServer.PrepareRun().RunWithContext(ctx)
}

func configForOptions(o *Options) (*workspaceapiserver.Config, error) {
	if err := o.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, []net.IP{netutils.ParseIPSloppy("127.0.0.1")}); err != nil {
		return nil, fmt.Errorf("error creating self-signed certificates: %v", err)
	}

	serverConfig := genericapiserver.NewRecommendedConfig(workspacescheme.Codecs)
	serverConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(generatedopenapi.GetOpenAPIDefinitions, openapi.NewDefinitionNamer(workspacescheme.Scheme))
	serverConfig.OpenAPIV3Config = genericapiserver.DefaultOpenAPIV3Config(generatedopenapi.GetOpenAPIDefinitions, openapi.NewDefinitionNamer(workspacescheme.Scheme))
	serverConfig.OpenAPIConfig.Info.Title = "karmada-workspace-apiserver"
	if err := o.ApplyTo(serverConfig); err != nil {
		return nil, err
	}
	serverConfig.Config.EffectiveVersion = compatibility.DefaultBuildEffectiveVersion()

	return &workspaceapiserver.Config{GenericConfig: serverConfig}, nil
}
