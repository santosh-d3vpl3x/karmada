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

package projection

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

const dnsLabelMaxLength = 63

// ProjectPodName returns a deterministic projected pod name for the runtime view.
// When more than one backing cluster contributes to the same logical pod, the name
// is stabilized with an opaque suffix that does not expose cluster identity.
func ProjectPodName(name string, clusterSet []string) string {
	clusters := uniqueSortedNonEmpty(clusterSet)
	if len(clusters) <= 1 {
		return name
	}

	suffix := stableSuffix(clusters)
	base := sanitizeDNSLabel(name)
	if base == "" {
		base = "pod"
	}

	maxBaseLength := dnsLabelMaxLength - len(suffix) - 1
	if maxBaseLength < 1 {
		maxBaseLength = 1
	}
	if len(base) > maxBaseLength {
		base = strings.TrimRight(base[:maxBaseLength], "-")
		if base == "" {
			base = "pod"
		}
	}

	projected := base + "-" + suffix
	if len(projected) > dnsLabelMaxLength {
		projected = strings.TrimRight(projected[:dnsLabelMaxLength], "-")
	}
	if projected == "" {
		return "pod"
	}
	return projected
}

// ProjectPod returns a workspace-visible copy of a pod with a stable logical name.
func ProjectPod(pod *corev1.Pod, clusterSet []string) *corev1.Pod {
	if pod == nil {
		return nil
	}

	projected := pod.DeepCopy()
	projected.Name = ProjectPodName(projected.Name, clusterSet)
	projected.UID = types.UID(stableUID(projected.Namespace, projected.Name))
	projected.ResourceVersion = ""
	return projected
}

// ProjectEvent returns a workspace-visible copy of an event with the provided logical object reference.
func ProjectEvent(event *corev1.Event, involvedObject corev1.ObjectReference) *corev1.Event {
	if event == nil {
		return nil
	}

	projected := event.DeepCopy()
	projected.InvolvedObject = involvedObject
	projected.UID = types.UID(stableUID(projected.Namespace, projected.Name, involvedObject.Namespace, involvedObject.Name, involvedObject.Kind))
	projected.ResourceVersion = ""
	return projected
}

func uniqueSortedNonEmpty(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	sort.Strings(unique)
	return unique
}

func stableSuffix(values []string) string {
	return stableUID(strings.Join(values, "\x00"))[:12]
}

func stableUID(parts ...string) string {
	hash := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(hash[:])
}

func sanitizeDNSLabel(value string) string {
	value = strings.ToLower(value)
	var builder strings.Builder
	builder.Grow(len(value))

	lastDash := false
	for i := 0; i < len(value); i++ {
		c := value[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			builder.WriteByte(c)
			lastDash = false
		default:
			if !lastDash {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}

	return strings.Trim(builder.String(), "-")
}
