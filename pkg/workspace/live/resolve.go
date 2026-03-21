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

package live

import (
	"errors"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// ErrNoLiveTarget indicates that the workspace index found no eligible target.
	ErrNoLiveTarget = errors.New("no eligible live target")
	// ErrAmbiguousLiveTarget indicates that the workspace index matched more than one target.
	ErrAmbiguousLiveTarget = errors.New("ambiguous live target")
)

// Request describes the logical pod live request to resolve.
type Request struct {
	Resource    schema.GroupVersionResource
	Namespace   string
	Name        string
	Subresource string
}

// Target describes one backing target selected from workspace state.
type Target struct {
	Cluster   string
	Namespace string
	Name      string
}

// State exposes the workspace index/projection snapshot used for live resolution.
type State interface {
	LiveTargets(Request) ([]Target, error)
}

// Resolver resolves live targets from a workspace state snapshot.
type Resolver interface {
	ResolveLiveTarget(Request) (Target, error)
}

type resolver struct {
	state State
}

// NewResolver returns a live target resolver backed by the provided state snapshot.
func NewResolver(state State) Resolver {
	return resolver{state: state}
}

func (r resolver) ResolveLiveTarget(req Request) (Target, error) {
	return ResolveLiveTarget(r.state, req)
}

// ResolveLiveTarget selects a unique live target from workspace state.
func ResolveLiveTarget(state State, req Request) (Target, error) {
	if state == nil {
		return Target{}, ErrNoLiveTarget
	}

	candidates, err := state.LiveTargets(req)
	if err != nil {
		return Target{}, err
	}

	switch len(candidates) {
	case 0:
		return Target{}, ErrNoLiveTarget
	case 1:
		return candidates[0], nil
	default:
		return Target{}, ErrAmbiguousLiveTarget
	}
}
