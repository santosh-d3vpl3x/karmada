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
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	genericrequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"

	workspacev1alpha1 "github.com/karmada-io/karmada/pkg/apis/workspace/v1alpha1"
	utiltest "github.com/karmada-io/karmada/pkg/util/testing"
	"github.com/karmada-io/karmada/pkg/workspace/live"
)

type fakeLiveResolver struct {
	target live.Target
	err    error
}

func (f fakeLiveResolver) ResolveLiveTarget(live.Request) (live.Target, error) {
	if f.err != nil {
		return live.Target{}, f.err
	}
	return f.target, nil
}

type fakeLiveConnector struct {
	called bool
	target live.Target
	req    live.Request
}

func (f *fakeLiveConnector) Connect(_ context.Context, target live.Target, req live.Request, _ runtime.Object, _ rest.Responder) (http.Handler, error) {
	f.called = true
	f.target = target
	f.req = req
	return http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(rw, fmt.Sprintf("%s/%s", target.Cluster, req.Subresource))
	}), nil
}

func newPodLiveRequest(t *testing.T, subresource string) *http.Request {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	req = req.WithContext(genericrequest.WithRequestInfo(req.Context(), &genericrequest.RequestInfo{
		IsResourceRequest: true,
		APIGroup:          "",
		APIVersion:        "v1",
		Resource:          "pods",
		Namespace:         "default",
		Name:              "demo",
		Subresource:       subresource,
	}))
	return req
}

func TestPodRESTConnectDelegatesUniqueTarget(t *testing.T) {
	connector := &fakeLiveConnector{}
	rest := NewPodREST(fakeLiveResolver{
		target: live.Target{Cluster: "member-a"},
	}, connector)

	req := newPodLiveRequest(t, "logs")
	resp := httptest.NewRecorder()

	h, err := rest.Connect(req.Context(), "demo", nil, utiltest.NewResponder(resp))
	if err != nil {
		t.Fatal(err)
	}

	h.ServeHTTP(resp, req)

	if !connector.called {
		t.Fatal("expected live connector to be invoked")
	}
	if connector.target.Cluster != "member-a" {
		t.Fatalf("got cluster %q, want %q", connector.target.Cluster, "member-a")
	}
	if connector.req.Name != "demo" || connector.req.Namespace != "default" || connector.req.Subresource != "logs" {
		t.Fatalf("unexpected resolved request: %#v", connector.req)
	}
	if got := resp.Body.String(); got != "member-a/logs" {
		t.Fatalf("got body %q, want %q", got, "member-a/logs")
	}
}

func TestPodRESTConnectReturnsConflictOnAmbiguousTarget(t *testing.T) {
	rest := NewPodREST(fakeLiveResolver{err: live.ErrAmbiguousLiveTarget}, &fakeLiveConnector{})

	req := newPodLiveRequest(t, "exec")
	_, err := rest.Connect(req.Context(), "demo", nil, utiltest.NewResponder(httptest.NewRecorder()))
	if !apierrors.IsConflict(err) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestPodRESTConnectReturnsNotFoundOnMissingTarget(t *testing.T) {
	rest := NewPodREST(fakeLiveResolver{err: live.ErrNoLiveTarget}, &fakeLiveConnector{})

	req := newPodLiveRequest(t, "attach")
	_, err := rest.Connect(req.Context(), "demo", nil, utiltest.NewResponder(httptest.NewRecorder()))
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestPodRESTRejectsUnsupportedSubresource(t *testing.T) {
	rest := NewPodREST(fakeLiveResolver{
		target: live.Target{Cluster: "member-a"},
	}, &fakeLiveConnector{})

	req := newPodLiveRequest(t, string(workspacev1alpha1.LiveSubresourceProxy))
	_, err := rest.Connect(req.Context(), "demo", nil, utiltest.NewResponder(httptest.NewRecorder()))
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected not found, got %v", err)
	}
}
