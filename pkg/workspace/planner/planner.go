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

package planner

import (
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"

	"github.com/karmada-io/karmada/pkg/workspace/support"
)

type UnsupportedFeature string

const (
	UnsupportedResource      UnsupportedFeature = "resource"
	UnsupportedVerb          UnsupportedFeature = "verb"
	UnsupportedFieldSelector UnsupportedFeature = "fieldSelector"
)

type QueryRequest struct {
	Resource      schema.GroupVersionResource
	Namespace     string
	Name          string
	LabelSelector string
	FieldSelector string
	Watch         bool
}

type PushdownQuery struct {
	Namespace     string
	Name          string
	LabelSelector string
	FieldSelector string
	Watch         bool
}

type Plan struct {
	CandidateClusters []string
	Pushdown          PushdownQuery
	Unsupported       []UnsupportedFeature
}

func (p Plan) Supported() bool {
	return len(p.Unsupported) == 0
}

func ConservativePushdown(req QueryRequest, candidateClusters []string) (Plan, error) {
	plan := Plan{
		CandidateClusters: append([]string(nil), candidateClusters...),
		Pushdown: PushdownQuery{
			Namespace: req.Namespace,
			Name:      req.Name,
			Watch:     req.Watch,
		},
	}
	sort.Strings(plan.CandidateClusters)

	if !support.Exposes(req.Resource) {
		plan.Unsupported = appendUniqueUnsupported(plan.Unsupported, UnsupportedResource)
		return plan, nil
	}

	if !support.Supports(req.Resource, requestVerb(req)) {
		plan.Unsupported = appendUniqueUnsupported(plan.Unsupported, UnsupportedVerb)
		return plan, nil
	}

	if req.LabelSelector != "" {
		if _, err := labels.Parse(req.LabelSelector); err != nil {
			return Plan{}, fmt.Errorf("parse label selector: %w", err)
		}
		plan.Pushdown.LabelSelector = req.LabelSelector
	}

	if req.FieldSelector == "" {
		return plan, nil
	}

	selector, err := fields.ParseSelector(req.FieldSelector)
	if err != nil {
		return Plan{}, fmt.Errorf("parse field selector: %w", err)
	}

	for _, requirement := range selector.Requirements() {
		if requirement.Value == "" {
			return Plan{}, fmt.Errorf("parse field selector: empty value for %s", requirement.Field)
		}

		if requirement.Operator != selection.Equals && requirement.Operator != selection.DoubleEquals {
			plan.Unsupported = appendUniqueUnsupported(plan.Unsupported, UnsupportedFieldSelector)
			continue
		}

		switch requirement.Field {
		case "metadata.name", "metadata.namespace":
			// Supported for conservative pushdown.
		default:
			plan.Unsupported = appendUniqueUnsupported(plan.Unsupported, UnsupportedFieldSelector)
		}
	}

	if plan.Supported() {
		plan.Pushdown.FieldSelector = req.FieldSelector
	}

	return plan, nil
}

func requestVerb(req QueryRequest) string {
	if req.Watch {
		return "watch"
	}
	if req.Name != "" {
		return "get"
	}
	return "list"
}

func appendUniqueUnsupported(in []UnsupportedFeature, feature UnsupportedFeature) []UnsupportedFeature {
	for _, existing := range in {
		if existing == feature {
			return in
		}
	}
	return append(in, feature)
}
