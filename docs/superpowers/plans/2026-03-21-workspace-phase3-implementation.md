# Workspace Phase 3 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the workspace framework to broader cluster-scoped resources, CRDs and arbitrary propagated resources with well-defined semantics, stronger aggregated or extension API behavior, and deeper runtime-fidelity improvements that were intentionally deferred from phases 1 and 2.

**Architecture:** Reuse the phase-1 and phase-2 workspace apiserver as the stable contract surface, then replace remaining hard-coded capability assumptions with a capability catalog that can describe built-in resources, cluster-scoped logical views, CRDs, and extension APIs without breaking truthful discovery. Karmada remains authoritative for desired state and propagation; phase 3 broadens what can be represented safely, not the control-plane ownership model.

**Tech Stack:** Go, genericapiserver, client-go, apiextensions-apiserver, apimachinery, Karmada propagation and binding APIs, generated clientset or dynamic clients where appropriate, Go tests, Ginkgo/Gomega e2e, `kubectl`, `ginkgo`

---

## Phase-3 Cut Line

This plan only covers the spec's Phase 3 and deferred-work items that belong there:

- broader cluster-scoped resources;
- CRDs and arbitrary propagated resources where semantics are well-defined;
- more complete extension and aggregated API behavior;
- deeper fidelity improvements where phases 1 and 2 intentionally stayed bounded.

This plan does not revisit the giant-cluster model or split desired-state ownership away from Karmada.

## File Structure

### Existing files to modify

- `pkg/workspace/support/matrix.go`: retire remaining phase-specific hard-coding that blocks broader propagated-resource support.
- `pkg/workspace/support/matrix_test.go`: preserve truthful-discovery guarantees while the catalog becomes more dynamic.
- `pkg/workspace/planner/planner.go`: extend query-planning decisions to broader built-in, cluster-scoped, CRD, and extension resources.
- `pkg/workspace/planner/planner_test.go`: lock planner behavior for the broader surface.
- `pkg/workspace/facade/sourceobjects.go`: generalize desired-state compilation for broader propagated resources.
- `pkg/workspace/facade/sourceobjects_test.go`: verify metadata and write translation for the broader surface.
- `pkg/workspace/apiserver.go`: integrate the dynamic capability catalog, storage wiring, and truthful discovery.
- `pkg/workspace/apiserver_test.go`: keep storage-map assembly and installation coverage aligned with the broader surface.
- `pkg/workspace/apiserver_contract_test.go`: extend black-box discovery, verb, watch, and error checks for CRDs and extension behavior.
- `pkg/workspace/index/types.go`: carry broader runtime and propagated-resource metadata when deeper fidelity needs it.
- `pkg/workspace/index/types_test.go`: lock the richer index contracts.
- `pkg/workspace/projection/runtime.go`: improve runtime fidelity without leaking backing-cluster identity into primary names.
- `pkg/workspace/projection/runtime_test.go`: verify deeper fidelity while preserving stable logical identity.
- `pkg/workspace/live/resolve.go`: keep live-target resolution consistent once broader resource coverage exists.
- `pkg/workspace/live/resolve_test.go`: lock resolver behavior under the deeper runtime model.
- `pkg/registry/workspace/storage/storage.go`: register the broadened storage catalog.
- `pkg/registry/workspace/storage/pods.go`: preserve projected-runtime semantics under the richer fidelity model.
- `pkg/registry/workspace/storage/events.go`: preserve event projection under the richer fidelity model.
- `pkg/registry/workspace/storage/pods_test.go`: lock projected pod behavior for the richer runtime model.
- `pkg/registry/workspace/storage/events_test.go`: lock projected event behavior for the richer runtime model.
- `test/e2e/suites/workspace/workspace_api_test.go`: extend compatibility coverage to the approved phase-3 surface.
- `test/e2e/suites/workspace/workspace_live_test.go`: extend live-runtime coverage where the phase-3 surface changes it.
- `test/e2e/suites/workspace/README.md`: update manual validation notes for the broader surface.

### New source files to create

- `pkg/workspace/support/catalog.go`: capability catalog for broader built-in resources, cluster-scoped resources, CRDs, and extension APIs.
- `pkg/workspace/support/catalog_test.go`: tests for catalog construction and truthful-discovery filtering.
- `pkg/workspace/facade/dynamic.go`: translation helpers for arbitrary propagated resources that still compile into Karmada-native desired state.
- `pkg/workspace/facade/dynamic_test.go`: tests for generic desired-state translation.
- `pkg/registry/workspace/storage/dynamic.go`: dynamic storage adapters for CRDs and arbitrary propagated resources with approved semantics.
- `pkg/registry/workspace/storage/dynamic_test.go`: tests for dynamic storage behavior.
- `pkg/registry/workspace/storage/extensions.go`: extension or aggregated API routing helpers for the approved phase-3 scope.
- `pkg/registry/workspace/storage/extensions_test.go`: tests for extension or aggregated API behavior.

## Tasks

### Task 1: Lock The Phase-3 Capability Envelope In Tests First

**Files:**
- Create: `pkg/workspace/support/catalog_test.go`
- Modify: `pkg/workspace/support/matrix.go`
- Modify: `pkg/workspace/support/matrix_test.go`
- Modify: `pkg/workspace/apiserver_contract_test.go`
- Modify: `test/e2e/suites/workspace/workspace_api_test.go`

- [ ] **Step 1: Write the failing capability-catalog and contract tests**

```go
func TestPhase3CatalogExposesOnlyApprovedResources(t *testing.T) {
	catalog := BuildCatalog(input)
	for _, gvr := range approvedPhase3Resources {
		if !catalog.Exposes(gvr) {
			t.Fatalf("missing %s", gvr)
		}
	}
}
```

- [ ] **Step 2: Run the capability-focused tests to verify they fail**

Run: `go test ./pkg/workspace/support ./pkg/workspace -run 'TestPhase3CatalogExposesOnlyApprovedResources|TestWorkspaceDiscoveryOnlyAdvertisesSupportedResources' -count=1`
Expected: FAIL because the broader capability envelope is not catalog-driven yet.

- [ ] **Step 3: Implement the minimal catalog scaffolding**

Build the catalog around approved phase-3 resources only. Do not advertise any CRD, cluster-scoped resource, or extension API until it has explicit semantics and tests.

- [ ] **Step 4: Re-run the catalog tests**

Run: `go test ./pkg/workspace/support ./pkg/workspace -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/workspace/support/catalog_test.go pkg/workspace/support/matrix.go pkg/workspace/support/matrix_test.go pkg/workspace/apiserver_contract_test.go test/e2e/suites/workspace/workspace_api_test.go
git commit -m "feat: add workspace phase3 capability catalog"
```

### Task 2: Generalize Desired-State Translation For Broader Propagated Resources

**Files:**
- Create: `pkg/workspace/facade/dynamic.go`
- Create: `pkg/workspace/facade/dynamic_test.go`
- Modify: `pkg/workspace/facade/sourceobjects.go`
- Modify: `pkg/workspace/facade/sourceobjects_test.go`
- Modify: `pkg/workspace/planner/planner.go`
- Modify: `pkg/workspace/planner/planner_test.go`

- [ ] **Step 1: Write the failing translation and planner tests**

```go
func TestCompileDesiredStateForDynamicResource(t *testing.T) {
	obj, err := CompileDynamicDesiredState(gvr, workspace, unstructuredObj)
	if err != nil {
		t.Fatal(err)
	}
	if obj.GetAnnotations()[AnnotationWorkspaceResource] != gvr.String() {
		t.Fatalf("missing resource annotation")
	}
}
```

- [ ] **Step 2: Run the facade and planner tests to verify they fail**

Run: `go test ./pkg/workspace/facade ./pkg/workspace/planner -run 'TestCompileDesiredStateForDynamicResource|TestPlanner.*' -count=1`
Expected: FAIL because generic propagated-resource translation is not implemented yet.

- [ ] **Step 3: Implement the minimal generic translation layer**

Keep the same ownership rule as phase 1: workspace writes still compile into Karmada-native desired state. Phase 3 expands the eligible resource shapes; it does not create a second persistence model.

- [ ] **Step 4: Re-run the translation and planner tests**

Run: `go test ./pkg/workspace/facade ./pkg/workspace/planner -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/workspace/facade/dynamic.go pkg/workspace/facade/dynamic_test.go pkg/workspace/facade/sourceobjects.go pkg/workspace/facade/sourceobjects_test.go pkg/workspace/planner/planner.go pkg/workspace/planner/planner_test.go
git commit -m "feat: generalize workspace desired-state translation"
```

### Task 3: Add Dynamic Storage And Truthful Discovery For CRDs And Extension APIs

**Files:**
- Create: `pkg/registry/workspace/storage/dynamic.go`
- Create: `pkg/registry/workspace/storage/dynamic_test.go`
- Create: `pkg/registry/workspace/storage/extensions.go`
- Create: `pkg/registry/workspace/storage/extensions_test.go`
- Modify: `pkg/registry/workspace/storage/storage.go`
- Modify: `pkg/workspace/apiserver.go`
- Modify: `pkg/workspace/apiserver_test.go`
- Modify: `pkg/workspace/apiserver_contract_test.go`
- Modify: `test/e2e/suites/workspace/workspace_api_test.go`

- [ ] **Step 1: Write the failing dynamic-storage and extension tests**

```go
func TestDynamicStorageAdvertisesApprovedCRDOnly(t *testing.T) {
	resources := discoverAPIResources(t, server, workspaceProxyPath("team-a", "/apis/example.io/v1"))
	if !containsResource(resources, "widgets") {
		t.Fatal("expected widgets in discovery")
	}
}
```

- [ ] **Step 2: Run the storage and contract tests to verify they fail**

Run: `go test ./pkg/registry/workspace/storage ./pkg/workspace -run 'TestDynamicStorageAdvertisesApprovedCRDOnly|TestWorkspaceDiscoveryOnlyAdvertisesSupportedResources' -count=1`
Expected: FAIL because dynamic CRD or extension routing is not wired yet.

- [ ] **Step 3: Implement dynamic storage and discovery registration minimally**

Only expose CRDs and extension APIs whose semantics are explicitly approved and testable at workspace scope. Unsupported CRDs must remain absent from discovery.

- [ ] **Step 4: Re-run storage and contract tests**

Run: `go test ./pkg/registry/workspace/storage ./pkg/workspace -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/registry/workspace/storage/dynamic.go pkg/registry/workspace/storage/dynamic_test.go pkg/registry/workspace/storage/extensions.go pkg/registry/workspace/storage/extensions_test.go pkg/registry/workspace/storage/storage.go pkg/workspace/apiserver.go pkg/workspace/apiserver_test.go pkg/workspace/apiserver_contract_test.go test/e2e/suites/workspace/workspace_api_test.go
git commit -m "feat: add workspace dynamic and extension storage"
```

### Task 4: Deepen Runtime Fidelity Without Breaking Logical Identity

**Files:**
- Modify: `pkg/workspace/index/types.go`
- Modify: `pkg/workspace/index/types_test.go`
- Modify: `pkg/workspace/projection/runtime.go`
- Modify: `pkg/workspace/projection/runtime_test.go`
- Modify: `pkg/workspace/live/resolve.go`
- Modify: `pkg/workspace/live/resolve_test.go`
- Modify: `pkg/registry/workspace/storage/pods.go`
- Modify: `pkg/registry/workspace/storage/pods_test.go`
- Modify: `pkg/registry/workspace/storage/events.go`
- Modify: `pkg/registry/workspace/storage/events_test.go`
- Modify: `test/e2e/suites/workspace/workspace_live_test.go`
- Modify: `test/e2e/suites/workspace/README.md`

- [ ] **Step 1: Write the failing runtime-fidelity tests**

```go
func TestProjectPodPreservesStableLogicalIdentityWithRicherRuntimeState(t *testing.T) {
	projected := ProjectPod(backingPod, []string{"member-a", "member-b"})
	if projected.Name == backingPod.Name {
		t.Fatal("expected stable opaque suffix when needed")
	}
}
```

- [ ] **Step 2: Run the runtime-focused tests to verify they fail**

Run: `go test ./pkg/workspace/projection ./pkg/workspace/live ./pkg/registry/workspace/storage -run 'TestProjectPodPreservesStableLogicalIdentityWithRicherRuntimeState|TestProjectedPod.*|TestProjectedEvent.*' -count=1`
Expected: FAIL because the richer runtime-fidelity model is not implemented yet.

- [ ] **Step 3: Implement the deeper runtime model minimally**

Improve fidelity only where the richer metadata is needed. Preserve the giant-cluster guarantees from earlier phases: stable logical identity, truthful discovery, explicit ambiguity, and no backing-cluster names in primary object names.

- [ ] **Step 4: Run package and e2e verification for the full phase-3 slice**

Run: `go test ./pkg/workspace/... ./pkg/registry/workspace/storage -count=1`
Expected: PASS.

Run: `go test ./test/e2e/suites/workspace -count=1`
Expected: PASS or SKIP outside a workspace-capable environment.

If the environment supports it:
Run: `ginkgo -v ./test/e2e/suites/workspace -- --poll-interval=5s --poll-timeout=5m`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/workspace/index/types.go pkg/workspace/index/types_test.go pkg/workspace/projection/runtime.go pkg/workspace/projection/runtime_test.go pkg/workspace/live/resolve.go pkg/workspace/live/resolve_test.go pkg/registry/workspace/storage/pods.go pkg/registry/workspace/storage/pods_test.go pkg/registry/workspace/storage/events.go pkg/registry/workspace/storage/events_test.go test/e2e/suites/workspace/workspace_live_test.go test/e2e/suites/workspace/README.md
git commit -m "feat: deepen workspace runtime fidelity"
```

## Verification Checklist

Run these before claiming phase-3 execution is complete:

- `go test ./pkg/workspace/... -count=1`
- `go test ./pkg/registry/workspace/storage -count=1`
- `go test ./pkg/karmadactl/workspace -count=1`
- `go test ./cmd/karmada-workspace-apiserver/... -count=1`
- `go test ./test/e2e/suites/workspace -count=1`
- `ginkgo -v ./test/e2e/suites/workspace -- --poll-interval=5s --poll-timeout=5m` when a workspace-capable environment is available
