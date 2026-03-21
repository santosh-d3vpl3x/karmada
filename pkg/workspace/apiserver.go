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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	genericrequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/util/compatibility"
	"k8s.io/klog/v2"

	workspacescheme "github.com/karmada-io/karmada/pkg/apis/workspace/scheme"
	workspacev1alpha1 "github.com/karmada-io/karmada/pkg/apis/workspace/v1alpha1"
	workspacestorage "github.com/karmada-io/karmada/pkg/registry/workspace/storage"
	"github.com/karmada-io/karmada/pkg/workspace/facade"
	"github.com/karmada-io/karmada/pkg/workspace/index"
	"github.com/karmada-io/karmada/pkg/workspace/live"
	"github.com/karmada-io/karmada/pkg/workspace/support"
)

const componentName = "karmada-workspace-apiserver"

var (
	coreV1  = schema.GroupVersion{Version: "v1"}
	appsV1  = appsv1.SchemeGroupVersion
	batchV1 = batchv1.SchemeGroupVersion
)

// RequestAuthorizer authorizes one nested workspace request.
type RequestAuthorizer func(string, *workspaceRequestInfo) error

// ExtraConfig holds custom apiserver config.
type ExtraConfig struct {
	RuntimeState           index.State
	LiveResolver           live.Resolver
	LiveConnector          workspacestorage.LiveConnector
	RequestAuthorizer      RequestAuthorizer
	DesiredStateBackendURL *url.URL
	DesiredStateTransport  http.RoundTripper
}

// Config defines the config for the workspace API server.
type Config struct {
	GenericConfig *genericapiserver.RecommendedConfig
	ExtraConfig   ExtraConfig
}

// APIServer contains state for the workspace apiserver.
type APIServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

type completedConfig struct {
	GenericConfig genericapiserver.CompletedConfig
	ExtraConfig   *ExtraConfig
}

// CompletedConfig embeds a private pointer that cannot be instantiated outside this package.
type CompletedConfig struct {
	*completedConfig
}

// workspaceRequestInfo captures one proxied logical workspace request.
type workspaceRequestInfo struct {
	Workspace         string
	Path              string
	Verb              string
	APIGroup          string
	APIVersion        string
	Resource          schema.GroupVersionResource
	Namespace         string
	Name              string
	Subresource       string
	IsResourceRequest bool
	IsWatch           bool
}

// Complete fills in any fields not set that are required to have valid data.
func (cfg *Config) Complete() CompletedConfig {
	c := completedConfig{
		GenericConfig: cfg.GenericConfig.Complete(),
		ExtraConfig:   &cfg.ExtraConfig,
	}
	c.GenericConfig.EffectiveVersion = compatibility.DefaultBuildEffectiveVersion()
	return CompletedConfig{&c}
}

var storageMapBuilder = func(optsGetter generic.RESTOptionsGetter, extra *ExtraConfig) (map[string]rest.Storage, error) {
	storage, err := workspacestorage.NewStorage(workspacescheme.Scheme, optsGetter, newWorkspaceProxyHandler(extra))
	if err != nil {
		return nil, err
	}

	return map[string]rest.Storage{
		workspacev1alpha1.ResourcePluralWorkspace:            storage.Workspaces,
		workspacev1alpha1.ResourcePluralWorkspace + "/proxy": storage.Workspaces,
		workspacev1alpha1.ResourcePluralPlacementView:        storage.PlacementViews,
	}, nil
}

var apiGroupInstaller = func(server *APIServer, apiGroupInfo *genericapiserver.APIGroupInfo) error {
	return server.GenericAPIServer.InstallAPIGroup(apiGroupInfo)
}

func (c completedConfig) New() (*APIServer, error) {
	genericServer, err := c.GenericConfig.New(componentName, genericapiserver.NewEmptyDelegate())
	if err != nil {
		return nil, err
	}

	server := &APIServer{GenericAPIServer: genericServer}
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(workspacev1alpha1.GroupName, workspacescheme.Scheme, workspacescheme.ParameterCodec, workspacescheme.Codecs)

	storageMap, err := storageMapBuilder(c.GenericConfig.RESTOptionsGetter, c.ExtraConfig)
	if err != nil {
		klog.Errorf("unable to create REST storage for a workspace resource due to %v, will die", err)
		return nil, err
	}
	apiGroupInfo.VersionedResourcesStorageMap[workspacev1alpha1.GroupVersion.Version] = storageMap

	if err := apiGroupInstaller(server, &apiGroupInfo); err != nil {
		return nil, err
	}
	return server, nil
}

type workspaceProxyHandler struct {
	runtimeState      index.State
	liveResolver      live.Resolver
	liveConnector     workspacestorage.LiveConnector
	requestAuthorizer RequestAuthorizer
	backendURL        *url.URL
	backendTransport  http.RoundTripper
}

func newWorkspaceProxyHandler(extra *ExtraConfig) workspacestorage.WorkspaceProxyHandler {
	if extra == nil {
		extra = &ExtraConfig{}
	}
	return &workspaceProxyHandler{
		runtimeState:      extra.RuntimeState,
		liveResolver:      extra.LiveResolver,
		liveConnector:     extra.LiveConnector,
		requestAuthorizer: extra.RequestAuthorizer,
		backendURL:        extra.DesiredStateBackendURL,
		backendTransport:  extra.DesiredStateTransport,
	}
}

func (h *workspaceProxyHandler) ConnectWorkspace(_ context.Context, workspaceName, proxyPath string, responder rest.Responder) (http.Handler, error) {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		info, err := parseWorkspaceRequestInfo(req.Method, proxyPath, req.URL.Query(), workspaceName)
		if err != nil {
			writeAPIError(rw, err)
			return
		}
		if !info.IsResourceRequest {
			h.serveDiscovery(rw, info)
			return
		}
		if h.requestAuthorizer != nil {
			if err := h.requestAuthorizer(workspaceName, info); err != nil {
				writeAPIError(rw, err)
				return
			}
		}
		h.serveResource(rw, req, info, responder)
	}), nil
}

func (h *workspaceProxyHandler) serveDiscovery(rw http.ResponseWriter, info *workspaceRequestInfo) {
	switch {
	case info.Path == "/api":
		writeJSON(rw, http.StatusOK, &metav1.APIVersions{
			TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "APIVersions"},
			Versions: []string{coreV1.Version},
		})
	case info.Path == "/api/v1":
		writeJSON(rw, http.StatusOK, &metav1.APIResourceList{
			TypeMeta:     metav1.TypeMeta{APIVersion: "v1", Kind: "APIResourceList"},
			GroupVersion: coreV1.String(),
			APIResources: discoveryResourcesFor(coreV1),
		})
	case info.Path == "/apis":
		writeJSON(rw, http.StatusOK, &metav1.APIGroupList{
			TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "APIGroupList"},
			Groups: []metav1.APIGroup{
				workspaceAPIGroup(appsV1),
				workspaceAPIGroup(batchV1),
			},
		})
	case info.Path == "/apis/apps":
		writeJSON(rw, http.StatusOK, workspaceAPIGroup(appsV1))
	case info.Path == "/apis/apps/v1":
		writeJSON(rw, http.StatusOK, &metav1.APIResourceList{
			TypeMeta:     metav1.TypeMeta{APIVersion: "v1", Kind: "APIResourceList"},
			GroupVersion: appsV1.String(),
			APIResources: discoveryResourcesFor(appsV1),
		})
	case info.Path == "/apis/batch":
		writeJSON(rw, http.StatusOK, workspaceAPIGroup(batchV1))
	case info.Path == "/apis/batch/v1":
		writeJSON(rw, http.StatusOK, &metav1.APIResourceList{
			TypeMeta:     metav1.TypeMeta{APIVersion: "v1", Kind: "APIResourceList"},
			GroupVersion: batchV1.String(),
			APIResources: discoveryResourcesFor(batchV1),
		})
	default:
		writeAPIError(rw, apierrors.NewNotFound(schema.GroupResource{}, info.Path))
	}
}

func (h *workspaceProxyHandler) serveResource(rw http.ResponseWriter, req *http.Request, info *workspaceRequestInfo, responder rest.Responder) {
	if info.Subresource != "" {
		h.serveSubresource(rw, req, info, responder)
		return
	}
	if !support.Exposes(info.Resource) {
		writeAPIError(rw, workspacestorage.NewUnsupportedResourceError(info.Resource.Resource))
		return
	}
	if !support.Supports(info.Resource, info.Verb) {
		writeAPIError(rw, workspacestorage.NewUnsupportedRequestError(info.Resource, info.Verb))
		return
	}
	if isProjectedRuntimeResource(info.Resource) {
		h.serveProjectedRuntime(rw, req, info)
		return
	}
	h.forwardDesiredState(rw, req, info)
}

func (h *workspaceProxyHandler) serveSubresource(rw http.ResponseWriter, req *http.Request, info *workspaceRequestInfo, responder rest.Responder) {
	if !support.Exposes(info.Resource) || !support.SupportsSubresource(info.Resource, info.Subresource) {
		writeAPIError(rw, workspacestorage.NewUnsupportedResourceError(info.Resource.Resource+"/"+info.Subresource))
		return
	}
	if info.Resource != corev1.SchemeGroupVersion.WithResource("pods") || info.Name == "" {
		writeAPIError(rw, workspacestorage.NewUnsupportedResourceError(info.Resource.Resource+"/"+info.Subresource))
		return
	}

	ctx := req.Context()
	if info.Namespace != "" {
		ctx = genericrequest.WithNamespace(ctx, info.Namespace)
	}
	ctx = genericrequest.WithRequestInfo(ctx, &genericrequest.RequestInfo{
		Namespace:   info.Namespace,
		Subresource: info.Subresource,
	})

	podREST := workspacestorage.NewPodREST(h.liveResolver, h.liveConnector)
	handler, err := podREST.Connect(ctx, info.Name, nil, responder)
	if err != nil {
		writeAPIError(rw, err)
		return
	}
	handler.ServeHTTP(rw, req.WithContext(ctx))
}

func (h *workspaceProxyHandler) serveProjectedRuntime(rw http.ResponseWriter, req *http.Request, info *workspaceRequestInfo) {
	ctx := req.Context()
	if info.Namespace != "" {
		ctx = genericrequest.WithNamespace(ctx, info.Namespace)
	}

	switch info.Resource {
	case corev1.SchemeGroupVersion.WithResource("pods"):
		h.serveProjectedStorage(rw, ctx, info, workspacestorage.NewProjectedPodREST(h.runtimeState))
	case corev1.SchemeGroupVersion.WithResource("events"):
		h.serveProjectedStorage(rw, ctx, info, workspacestorage.NewProjectedEventREST(h.runtimeState))
	default:
		writeAPIError(rw, workspacestorage.NewUnsupportedResourceError(info.Resource.Resource))
	}
}

func (h *workspaceProxyHandler) serveProjectedStorage(rw http.ResponseWriter, ctx context.Context, info *workspaceRequestInfo, storage runtimeStorage) {
	if info.IsWatch {
		watcher, err := storage.Watch(ctx, &metainternalversion.ListOptions{ResourceVersion: ""})
		if err != nil {
			writeAPIError(rw, err)
			return
		}
		if watcher != nil {
			watcher.Stop()
		}
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusOK)
		return
	}

	if info.Name != "" {
		obj, err := storage.Get(ctx, info.Name, &metav1.GetOptions{})
		if err != nil {
			writeAPIError(rw, err)
			return
		}
		writeJSON(rw, http.StatusOK, obj)
		return
	}

	obj, err := storage.List(ctx, &metainternalversion.ListOptions{})
	if err != nil {
		writeAPIError(rw, err)
		return
	}
	writeJSON(rw, http.StatusOK, obj)
}

func (h *workspaceProxyHandler) forwardDesiredState(rw http.ResponseWriter, req *http.Request, info *workspaceRequestInfo) {
	if h.backendURL == nil {
		writeAPIError(rw, fmt.Errorf("desired-state backend is not configured"))
		return
	}

	body, err := prepareDesiredStateBody(req, info)
	if err != nil {
		writeAPIError(rw, err)
		return
	}

	target := *h.backendURL
	target.Path = joinURLPath(target.Path, info.Path)
	query := req.URL.Query()
	if info.Name == "" && (info.Verb == "list" || info.Verb == "watch") {
		query = cloneValues(query)
		query.Set("labelSelector", mergeLabelSelector(query.Get("labelSelector"), facade.LabelWorkspaceName+"="+info.Workspace))
	}
	target.RawQuery = query.Encode()

	outbound, err := http.NewRequestWithContext(req.Context(), req.Method, target.String(), bytes.NewReader(body))
	if err != nil {
		writeAPIError(rw, err)
		return
	}
	copyHeaders(outbound.Header, req.Header)
	outbound.ContentLength = int64(len(body))

	client := &http.Client{Transport: h.backendTransport}
	resp, err := client.Do(outbound)
	if err != nil {
		writeAPIError(rw, err)
		return
	}
	defer resp.Body.Close()

	copyHeaders(rw.Header(), resp.Header)
	rw.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(rw, resp.Body)
}

func prepareDesiredStateBody(req *http.Request, info *workspaceRequestInfo) ([]byte, error) {
	if req.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	_ = req.Body.Close()
	if len(body) == 0 || !requiresDesiredStateCompilation(info.Verb) || !strings.Contains(strings.ToLower(req.Header.Get("Content-Type")), "json") {
		return body, nil
	}

	obj := &unstructured.Unstructured{}
	if err := obj.UnmarshalJSON(body); err != nil {
		return body, nil
	}

	compiled, err := facade.CompileDesiredState(info.Resource, &workspacev1alpha1.Workspace{ObjectMeta: metav1.ObjectMeta{Name: info.Workspace}}, obj)
	if err != nil {
		return nil, err
	}
	return json.Marshal(compiled)
}

func parseWorkspaceRequestInfo(method, proxyPath string, query url.Values, workspaceName string) (*workspaceRequestInfo, error) {
	cleanedPath := path.Clean("/" + strings.TrimPrefix(proxyPath, "/"))
	if cleanedPath == "." {
		cleanedPath = "/"
	}
	segments := splitPath(cleanedPath)
	info := &workspaceRequestInfo{Workspace: workspaceName, Path: cleanedPath}
	if len(segments) == 0 {
		return info, nil
	}

	switch segments[0] {
	case "api":
		if len(segments) == 1 {
			return info, nil
		}
		info.APIVersion = segments[1]
		if len(segments) == 2 {
			return info, nil
		}
		info.IsResourceRequest = true
		return populateWorkspaceResourceInfo(info, method, query, segments[2:])
	case "apis":
		if len(segments) == 1 {
			return info, nil
		}
		info.APIGroup = segments[1]
		if len(segments) == 2 {
			return info, nil
		}
		info.APIVersion = segments[2]
		if len(segments) == 3 {
			return info, nil
		}
		info.IsResourceRequest = true
		return populateWorkspaceResourceInfo(info, method, query, segments[3:])
	default:
		return nil, apierrors.NewNotFound(schema.GroupResource{}, cleanedPath)
	}
}

func populateWorkspaceResourceInfo(info *workspaceRequestInfo, method string, query url.Values, segments []string) (*workspaceRequestInfo, error) {
	if len(segments) == 0 {
		return nil, apierrors.NewNotFound(schema.GroupResource{}, info.Path)
	}

	resource := ""
	hasName := false
	if segments[0] == "namespaces" {
		if len(segments) <= 2 {
			resource = "namespaces"
			if len(segments) == 2 {
				info.Name = segments[1]
				hasName = true
			}
		} else {
			info.Namespace = segments[1]
			resource = segments[2]
			if len(segments) >= 4 {
				info.Name = segments[3]
				hasName = true
			}
			if len(segments) >= 5 {
				info.Subresource = strings.ToLower(segments[4])
			}
		}
	} else {
		resource = segments[0]
		if len(segments) >= 2 {
			info.Name = segments[1]
			hasName = true
		}
		if len(segments) >= 3 {
			info.Subresource = strings.ToLower(segments[2])
		}
	}

	info.Resource = schema.GroupVersionResource{Group: info.APIGroup, Version: info.APIVersion, Resource: resource}
	info.IsWatch = watchRequested(query)
	info.Verb = requestVerb(method, hasName, info.Subresource != "", info.IsWatch)
	return info, nil
}

func discoveryResourcesFor(groupVersion schema.GroupVersion) []metav1.APIResource {
	resources := make([]metav1.APIResource, 0)
	for _, gvr := range support.Resources() {
		if gvr.Group != groupVersion.Group || gvr.Version != groupVersion.Version {
			continue
		}
		capability, ok := support.CapabilityFor(gvr)
		if !ok {
			continue
		}
		resources = append(resources, metav1.APIResource{
			Name:         gvr.Resource,
			SingularName: singularNameFor(gvr),
			Namespaced:   namespacedResource(gvr),
			Kind:         kindFor(gvr),
			Verbs:        discoveryVerbs(capability),
		})
		if gvr == corev1.SchemeGroupVersion.WithResource("pods") {
			resources = append(resources, discoverySubresources(gvr, capability)...)
		}
	}
	sort.Slice(resources, func(i, j int) bool {
		return resources[i].Name < resources[j].Name
	})
	return resources
}

func discoverySubresources(gvr schema.GroupVersionResource, capability support.Capability) []metav1.APIResource {
	subresources := capability.ConnectSubs.UnsortedList()
	sort.Strings(subresources)
	resources := make([]metav1.APIResource, 0, len(subresources))
	for _, subresource := range subresources {
		verbs := []string{"get"}
		if subresource != "logs" {
			verbs = []string{"create", "get"}
		}
		resources = append(resources, metav1.APIResource{
			Name:       gvr.Resource + "/" + subresource,
			Namespaced: true,
			Kind:       kindFor(gvr),
			Verbs:      verbs,
		})
	}
	return resources
}

func workspaceAPIGroup(groupVersion schema.GroupVersion) metav1.APIGroup {
	return metav1.APIGroup{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "APIGroup"},
		Name:     groupVersion.Group,
		PreferredVersion: metav1.GroupVersionForDiscovery{
			GroupVersion: groupVersion.String(),
			Version:      groupVersion.Version,
		},
		Versions: []metav1.GroupVersionForDiscovery{{
			GroupVersion: groupVersion.String(),
			Version:      groupVersion.Version,
		}},
	}
}

func discoveryVerbs(capability support.Capability) []string {
	verbs := make([]string, 0, 7)
	if capability.Read {
		verbs = append(verbs, "get", "list")
	}
	if capability.Watch {
		verbs = append(verbs, "watch")
	}
	if capability.Write {
		verbs = append(verbs, "create", "update", "patch", "delete", "deletecollection")
	}
	return verbs
}

func singularNameFor(gvr schema.GroupVersionResource) string {
	switch gvr.Resource {
	case "configmaps":
		return "configmap"
	case "daemonsets":
		return "daemonset"
	case "statefulsets":
		return "statefulset"
	case "cronjobs":
		return "cronjob"
	default:
		return strings.TrimSuffix(gvr.Resource, "s")
	}
}

func namespacedResource(gvr schema.GroupVersionResource) bool {
	return gvr != corev1.SchemeGroupVersion.WithResource("namespaces")
}

func kindFor(gvr schema.GroupVersionResource) string {
	switch gvr {
	case corev1.SchemeGroupVersion.WithResource("namespaces"):
		return "Namespace"
	case corev1.SchemeGroupVersion.WithResource("configmaps"):
		return "ConfigMap"
	case corev1.SchemeGroupVersion.WithResource("secrets"):
		return "Secret"
	case corev1.SchemeGroupVersion.WithResource("services"):
		return "Service"
	case corev1.SchemeGroupVersion.WithResource("pods"):
		return "Pod"
	case corev1.SchemeGroupVersion.WithResource("events"):
		return "Event"
	case appsv1.SchemeGroupVersion.WithResource("deployments"):
		return "Deployment"
	case appsv1.SchemeGroupVersion.WithResource("statefulsets"):
		return "StatefulSet"
	case appsv1.SchemeGroupVersion.WithResource("daemonsets"):
		return "DaemonSet"
	case batchv1.SchemeGroupVersion.WithResource("jobs"):
		return "Job"
	case batchv1.SchemeGroupVersion.WithResource("cronjobs"):
		return "CronJob"
	default:
		return "Object"
	}
}

func requestVerb(method string, hasName, hasSubresource, isWatch bool) string {
	if hasSubresource {
		return "connect"
	}
	if isWatch {
		return "watch"
	}
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead:
		if hasName {
			return "get"
		}
		return "list"
	case http.MethodPost:
		return "create"
	case http.MethodPut:
		return "update"
	case http.MethodPatch:
		return "patch"
	case http.MethodDelete:
		if hasName {
			return "delete"
		}
		return "deletecollection"
	default:
		return strings.ToLower(method)
	}
}

func watchRequested(query url.Values) bool {
	watch := strings.ToLower(query.Get("watch"))
	return watch == "1" || watch == "true"
}

func splitPath(p string) []string {
	trimmed := strings.Trim(p, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func joinURLPath(basePath, suffix string) string {
	if basePath == "" {
		return suffix
	}
	return path.Join(basePath, suffix)
}

func mergeLabelSelector(existing, workspaceSelector string) string {
	if existing == "" {
		return workspaceSelector
	}
	return existing + "," + workspaceSelector
}

func cloneValues(in url.Values) url.Values {
	out := make(url.Values, len(in))
	for key, values := range in {
		copied := make([]string, len(values))
		copy(copied, values)
		out[key] = copied
	}
	return out
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		copied := make([]string, len(values))
		copy(copied, values)
		dst[key] = copied
	}
}

func requiresDesiredStateCompilation(verb string) bool {
	switch verb {
	case "create", "update", "patch":
		return true
	default:
		return false
	}
}

func isProjectedRuntimeResource(resource schema.GroupVersionResource) bool {
	return resource == corev1.SchemeGroupVersion.WithResource("pods") || resource == corev1.SchemeGroupVersion.WithResource("events")
}

func writeJSON(rw http.ResponseWriter, statusCode int, object any) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(statusCode)
	_ = json.NewEncoder(rw).Encode(object)
}

func writeAPIError(rw http.ResponseWriter, err error) {
	status := metav1.Status{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Status"},
		Status:   metav1.StatusFailure,
		Code:     http.StatusInternalServerError,
		Reason:   metav1.StatusReasonInternalError,
		Message:  err.Error(),
	}
	if apiStatus, ok := err.(apierrors.APIStatus); ok {
		status = apiStatus.Status()
		status.Kind = "Status"
		status.APIVersion = "v1"
	}
	writeJSON(rw, int(status.Code), &status)
}

type runtimeStorage interface {
	Get(context.Context, string, *metav1.GetOptions) (runtime.Object, error)
	List(context.Context, *metainternalversion.ListOptions) (runtime.Object, error)
	Watch(context.Context, *metainternalversion.ListOptions) (watch.Interface, error)
}
