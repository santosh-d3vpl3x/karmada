# Workspace Phase 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expand the workspace surface beyond phase 1 with broader built-in coverage, selected cluster-scoped views with clear logical semantics, explicit support for ambiguous live-target inspection or selection, and stronger placement/runtime debug affordances.

**Architecture:** Keep the phase-1 workspace apiserver, support matrix, and Karmada-native desired-state pipeline as the stable core. Phase 2 extends truthful discovery and storage plumbing without changing the default giant-cluster model: normal CRUD still compiles into Karmada-owned desired state, projected runtime resources remain index-backed, and ambiguous live operations stay explicit unless the client opts into the new target-selection or inspection surface.

**Tech Stack:** Go, Cobra, genericapiserver, client-go, apimachinery, Karmada API conventions, generated clientset/listers/informers/applyconfigurations/openapi, Go tests, Ginkgo/Gomega e2e, `kubectl`, `ginkgo`

---

## Phase-2 Cut Line

This plan only covers the spec's Phase 2 and deferred-work items that belong there:

- broader built-in API coverage for additional namespaced resources;
- selected cluster-scoped views with clear logical semantics;
- explicit target-selection or inspection support for ambiguous runtime operations;
- stronger placement and runtime debugging surfaces.

This plan does not implement Phase 3 items such as arbitrary CRDs, arbitrary propagated resources, or full aggregated API fidelity.

## Current Approved Execution Cut

The Phase-2 spec is broader than the currently approved implementation slice. To avoid inventing unsupported scope, current execution on this branch is intentionally narrowed to:

- explicit target-selection or inspection support for ambiguous live pod operations;
- stronger placement and runtime debugging through a readable `PlacementView` surface and matching verification.

Implemented on the current branch:

- `86eea7464` `feat: add workspace live target inspection and debug views`
- `843422d81` `test: cover workspace phase2 debug e2e`

Also present on the current branch, but not part of the Phase-2 scope decision itself:

- `b6bd40998` `fix: restore workspace namespace CRUD contract`
- `3fa20b6e2` `fix: cover named workspace namespace writes`

Those follow-up commits repair and verify the Phase-1 namespace contract; they should not be mistaken for completion of the remaining Phase-2 resource-surface roadmap.

Still Phase-2 roadmap items, but not yet approved for implementation on this branch:

- additional built-in namespaced resources;
- selected cluster-scoped views with clear logical semantics.

Those remaining items require an explicit resource list decision before implementation so discovery and advertised support remain truthful.

## File Structure

### Existing files to modify

- `pkg/workspace/support/matrix.go`: extend the advertised capability set beyond phase 1.
- `pkg/workspace/support/matrix_test.go`: lock the phase-2 capability surface and truthful-discovery expectations.
- `pkg/workspace/apiserver.go`: expose the new phase-2 resources and live-selection hooks through the workspace proxy.
- `pkg/workspace/apiserver_test.go`: keep storage-map and installation coverage aligned with the expanded surface.
- `pkg/workspace/apiserver_contract_test.go`: extend black-box discovery, verb, watch, and live-subresource contract checks.
- `pkg/workspace/live/resolve.go`: preserve default ambiguity behavior while honoring explicit inspection or selection inputs.
- `pkg/workspace/live/resolve_test.go`: lock resolver behavior for default ambiguity and explicit target-selection paths.
- `pkg/registry/workspace/storage/pod_live.go`: plumb explicit target-selection or inspection through live pod subresources.
- `pkg/registry/workspace/storage/pod_live_test.go`: verify query handling, error shapes, and explicit target behavior.
- `pkg/registry/workspace/storage/placementview.go`: deepen the read-only debug surface.
- `pkg/registry/workspace/storage/placementview_test.go`: lock richer placement and ambiguity debug output.
- `pkg/apis/workspace/v1alpha1/types.go`: evolve placement or debug API fields only if needed for the approved phase-2 surface.
- `pkg/apis/workspace/v1alpha1/types_test.go`: lock any public API additions required by phase 2.
- `test/e2e/framework/workspace.go`: extend helpers only when the new kubectl and debug flows need it.
- `test/e2e/suites/workspace/workspace_api_test.go`: add broader discovery, cluster-scoped-view, and debug compatibility coverage.
- `test/e2e/suites/workspace/workspace_live_test.go`: add explicit target-selection or inspection coverage.
- `test/e2e/suites/workspace/README.md`: update the manual compatibility checklist for the phase-2 surface.

### New source files to create

- `pkg/registry/workspace/storage/cluster_scoped.go`: shared storage helpers for the selected cluster-scoped logical views.
- `pkg/registry/workspace/storage/cluster_scoped_test.go`: tests for the selected cluster-scoped read semantics.
- `pkg/workspace/live/selection.go`: explicit target-selection or inspection parsing that keeps the default ambiguity contract intact.
- `pkg/workspace/live/selection_test.go`: tests for selection parsing and validation.

## Tasks

### Task 1: Lock The Approved Phase-2 Surface In Tests First

**Files:**
- Modify: `pkg/workspace/support/matrix.go`
- Modify: `pkg/workspace/support/matrix_test.go`
- Modify: `pkg/workspace/apiserver_contract_test.go`
- Modify: `test/e2e/suites/workspace/workspace_api_test.go`
- Modify: `test/e2e/suites/workspace/README.md`

- [ ] **Step 1: Write the failing support-matrix and contract tests**

```go
func TestPhase2DiscoverySurface(t *testing.T) {
	for _, resource := range []string{
		// Fill this list only with the additional built-in namespaced resources
		// and selected cluster-scoped views approved for phase 2.
	} {
		if !containsResource(discoverWorkspaceResources(t, server), resource) {
			t.Fatalf("missing %s", resource)
		}
	}
}
```

- [ ] **Step 2: Run the smallest relevant tests to verify they fail**

Run: `go test ./pkg/workspace ./pkg/workspace/support -run 'TestPhase2DiscoverySurface|TestPhase1.*|TestWorkspaceDiscoveryOnlyAdvertisesSupportedResources' -count=1`
Expected: FAIL because the additional phase-2 resources are not yet advertised or test-locked.

- [ ] **Step 3: Extend the phase-2 capability surface minimally**

Implement only the approved phase-2 additions in `pkg/workspace/support/matrix.go` and discovery plumbing. Do not widen discovery beyond the reviewed list.

- [ ] **Step 4: Re-run the discovery-focused tests**

Run: `go test ./pkg/workspace ./pkg/workspace/support -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/workspace/support/matrix.go pkg/workspace/support/matrix_test.go pkg/workspace/apiserver_contract_test.go test/e2e/suites/workspace/workspace_api_test.go test/e2e/suites/workspace/README.md
git commit -m "feat: expand workspace phase2 discovery surface"
```

### Task 2: Add Selected Cluster-Scoped Logical Views

**Files:**
- Create: `pkg/registry/workspace/storage/cluster_scoped.go`
- Create: `pkg/registry/workspace/storage/cluster_scoped_test.go`
- Modify: `pkg/workspace/apiserver.go`
- Modify: `pkg/workspace/apiserver_test.go`
- Modify: `pkg/workspace/apiserver_contract_test.go`
- Modify: `test/e2e/suites/workspace/workspace_api_test.go`

- [ ] **Step 1: Write the failing storage and contract tests**

```go
func TestSelectedClusterScopedViewList(t *testing.T) {
	obj, err := storage.List(ctx, &metainternalversion.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := obj.(*corev1.NamespaceList); !ok {
		t.Fatalf("unexpected type %T", obj)
	}
}
```

- [ ] **Step 2: Run the cluster-scoped-view tests to verify they fail**

Run: `go test ./pkg/registry/workspace/storage ./pkg/workspace -run 'TestSelectedClusterScopedViewList|TestWorkspaceDiscoveryOnlyAdvertisesSupportedResources' -count=1`
Expected: FAIL because the selected cluster-scoped logical views do not exist yet.

- [ ] **Step 3: Implement the cluster-scoped storage helpers and wiring**

Build only the approved cluster-scoped views with clear logical semantics. Keep them truthful, read-only unless the approved surface explicitly requires writes, and omit them from discovery until they satisfy workspace-scoped semantics.

- [ ] **Step 4: Re-run storage and contract tests**

Run: `go test ./pkg/registry/workspace/storage ./pkg/workspace -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/registry/workspace/storage/cluster_scoped.go pkg/registry/workspace/storage/cluster_scoped_test.go pkg/workspace/apiserver.go pkg/workspace/apiserver_test.go pkg/workspace/apiserver_contract_test.go test/e2e/suites/workspace/workspace_api_test.go
git commit -m "feat: add workspace phase2 cluster-scoped views"
```

### Task 3: Add Explicit Target Selection Or Inspection For Ambiguous Live Requests

**Files:**
- Create: `pkg/workspace/live/selection.go`
- Create: `pkg/workspace/live/selection_test.go`
- Modify: `pkg/workspace/live/resolve.go`
- Modify: `pkg/workspace/live/resolve_test.go`
- Modify: `pkg/registry/workspace/storage/pod_live.go`
- Modify: `pkg/registry/workspace/storage/pod_live_test.go`
- Modify: `pkg/workspace/apiserver_contract_test.go`
- Modify: `test/e2e/suites/workspace/workspace_live_test.go`

- [ ] **Step 1: Write the failing resolver and storage tests**

```go
func TestResolveLiveTargetHonorsExplicitSelection(t *testing.T) {
	target, err := ResolveLiveTarget(state, Request{Name: "demo", Subresource: "logs"}, Selection{Cluster: "member-a"})
	if err != nil {
		t.Fatal(err)
	}
	if target.Cluster != "member-a" {
		t.Fatalf("got %s", target.Cluster)
	}
}
```

- [ ] **Step 2: Run the live-selection tests to verify they fail**

Run: `go test ./pkg/workspace/live ./pkg/registry/workspace/storage ./pkg/workspace -run 'TestResolveLiveTargetHonorsExplicitSelection|TestPodLiveConnect.*|TestWorkspacePodLiveAmbiguityReturnsConflict' -count=1`
Expected: FAIL because explicit selection or inspection is not implemented yet.

- [ ] **Step 3: Implement explicit target-selection or inspection without weakening defaults**

Preserve the existing rule: unqualified ambiguous requests still return `409 Conflict`. Only explicit client input may disambiguate or request an inspection response.

- [ ] **Step 4: Re-run the live-routing tests**

Run: `go test ./pkg/workspace/live ./pkg/registry/workspace/storage ./pkg/workspace -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/workspace/live/selection.go pkg/workspace/live/selection_test.go pkg/workspace/live/resolve.go pkg/workspace/live/resolve_test.go pkg/registry/workspace/storage/pod_live.go pkg/registry/workspace/storage/pod_live_test.go pkg/workspace/apiserver_contract_test.go test/e2e/suites/workspace/workspace_live_test.go
git commit -m "feat: add explicit workspace live target selection"
```

### Task 4: Strengthen Placement And Runtime Debug Surfaces

**Files:**
- Modify: `pkg/apis/workspace/v1alpha1/types.go`
- Modify: `pkg/apis/workspace/v1alpha1/types_test.go`
- Modify: `pkg/registry/workspace/storage/placementview.go`
- Modify: `pkg/registry/workspace/storage/placementview_test.go`
- Modify: `pkg/workspace/apiserver_contract_test.go`
- Modify: `test/e2e/suites/workspace/workspace_api_test.go`
- Modify: `test/e2e/suites/workspace/README.md`

- [ ] **Step 1: Write the failing placement/debug tests**

```go
func TestPlacementViewIncludesAmbiguityAndResolutionDetails(t *testing.T) {
	view, err := rest.Get(ctx, "apps.v1.default.deployments.demo", &metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(view.(*workspacev1alpha1.PlacementView).Status.EligibleClusters) == 0 {
		t.Fatal("expected eligible clusters")
	}
}
```

- [ ] **Step 2: Run the debug-surface tests to verify they fail**

Run: `go test ./pkg/apis/workspace/v1alpha1 ./pkg/registry/workspace/storage ./pkg/workspace -run 'TestPlacementViewIncludesAmbiguityAndResolutionDetails|TestPlacementView.*' -count=1`
Expected: FAIL because the richer placement or runtime debug data is not exposed yet.

- [ ] **Step 3: Implement the richer read-only debug surface**

Keep `PlacementView` secondary, RBAC-protected, and truthful. Do not leak backing-cluster identity into primary object names or the default happy path.

- [ ] **Step 4: Run package and e2e verification for the full phase-2 slice**

Run: `go test ./pkg/workspace/... ./pkg/registry/workspace/storage ./pkg/apis/workspace/v1alpha1 -count=1`
Expected: PASS.

Run: `go test ./test/e2e/suites/workspace -count=1`
Expected: PASS or SKIP outside a workspace-capable environment.

If the environment supports it:
Run: `ginkgo -v ./test/e2e/suites/workspace -- --poll-interval=5s --poll-timeout=5m`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/apis/workspace/v1alpha1/types.go pkg/apis/workspace/v1alpha1/types_test.go pkg/registry/workspace/storage/placementview.go pkg/registry/workspace/storage/placementview_test.go pkg/workspace/apiserver_contract_test.go test/e2e/suites/workspace/workspace_api_test.go test/e2e/suites/workspace/README.md
git commit -m "feat: strengthen workspace debug surfaces"
```

## Verification Checklist

Run these before claiming phase-2 execution is complete:

- `go test ./pkg/workspace/... -count=1`
- `go test ./pkg/registry/workspace/storage -count=1`
- `go test ./pkg/apis/workspace/v1alpha1 -count=1`
- `go test ./pkg/karmadactl/workspace -count=1`
- `go test ./cmd/karmada-workspace-apiserver/... -count=1`
- `go test ./test/e2e/suites/workspace -count=1`
- `ginkgo -v ./test/e2e/suites/workspace -- --poll-interval=5s --poll-timeout=5m` when a workspace-capable environment is available
