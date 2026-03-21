# Workspace Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the phase-1 tenant-facing workspace API so a workspace behaves like one bounded Kubernetes cluster view, with truthful discovery, Karmada-native desired-state writes, projected pod and event reads, live pod subresource routing, and explicit behavior verification for supported clients.

**Architecture:** Add a dedicated `karmada-workspace-apiserver` that exposes the `workspace.karmada.io` API group and a tenant-facing REST facade over the main Karmada apiserver. Writable resources compile into ordinary source objects in the main Karmada apiserver, while projected runtime resources and live subresources are served from workspace-specific projection and routing code. Keep Karmada authoritative for scheduling, binding, propagation, and execution, and validate the supported surface through package tests, contract tests, and workspace-specific e2e.

**Tech Stack:** Go, Cobra, genericapiserver, client-go, apiextensions/apimachinery, Karmada API conventions, generated clientset/listers/informers/applyconfigurations/openapi, Go tests, Ginkgo/Gomega e2e, `kubectl`, `hack/update-codegen.sh`, `hack/verify-codegen.sh`

---

## Phase-1 Cut Line

This plan only implements the phase-1 contract from [2026-03-21-workspace-giant-cluster-design.md](/data/data/com.termux/files/home/karmada/docs/superpowers/specs/2026-03-21-workspace-giant-cluster-design.md):

- Writable desired-state resources: `namespaces`, `configmaps`, `secrets`, `services`, `deployments`, `statefulsets`, `daemonsets`, `jobs`, `cronjobs`
- Projected read-only resources: `pods`, `events`
- Live pod subresources: `logs`, `exec`, `attach`, `portforward`
- Secondary debug surface: `PlacementView`

Do not pull cluster-scoped policy APIs, arbitrary CRDs, generic live targeting, or multi-cluster write semantics into this phase.

## Planning Status

- This document is the execution-ready implementation plan for Phase 1 only.
- Later phases remain explicitly recorded in `docs/superpowers/specs/2026-03-21-workspace-giant-cluster-design.md`, but they are not task-planned here on purpose.
- When Phase 1 is complete and validated, create separate implementation plans for Phase 2 and Phase 3 instead of extending this file ad hoc.
- Omitted task-level detail for later phases is intentional roadmap staging, not scope removal.

## File Structure

### Existing files to modify

- `pkg/apis/workspace/v1alpha1/types.go`: finalize the phase-1 `Workspace` and `PlacementView` API contracts.
- `pkg/apis/workspace/v1alpha1/types_test.go`: lock API enum values, JSON names, and public constants.
- `pkg/apis/workspace/v1alpha1/register.go`: register any added workspace types.
- `pkg/apis/workspace/install/install.go`: install the full workspace API group into schemes.
- `pkg/apis/workspace/scheme/register.go`: expose workspace scheme and codecs.
- `hack/update-codegen.sh`: add workspace APIs to deepcopy, openapi, applyconfiguration, clientset, lister, and informer generation.
- `pkg/karmadactl/karmadactl.go`: register the new `workspace` command group.
- `pkg/workspace/planner/planner.go`: keep query pushdown conservative and aligned with phase-1 support.
- `pkg/workspace/planner/planner_test.go`: keep planner behavior stable while phase-1 support is added.

### New source files to create

- `cmd/karmada-workspace-apiserver/main.go`: binary entrypoint.
- `cmd/karmada-workspace-apiserver/app/karmada-workspace-apiserver.go`: Cobra command wiring.
- `cmd/karmada-workspace-apiserver/app/options/options.go`: options, config construction, and `Run`.
- `cmd/karmada-workspace-apiserver/app/options/options_test.go`: options validation and defaults.
- `pkg/workspace/apiserver.go`: workspace API group installation and server assembly.
- `pkg/workspace/apiserver_test.go`: API group install and storage-map tests.
- `pkg/workspace/apiserver_contract_test.go`: black-box contract tests for discovery, verbs, watch, and error semantics.
- `pkg/workspace/index/types.go`: internal contracts for projected object snapshots and watch events.
- `pkg/workspace/index/types_test.go`: index contract tests.
- `pkg/workspace/support/matrix.go`: single source of truth for the phase-1 support matrix.
- `pkg/workspace/support/matrix_test.go`: support-matrix tests.
- `pkg/workspace/facade/metadata.go`: shared workspace labels and annotations used on compiled source objects.
- `pkg/workspace/facade/sourceobjects.go`: compile workspace writes into main-Karmada source objects.
- `pkg/workspace/facade/sourceobjects_test.go`: translation and metadata tests.
- `pkg/workspace/projection/runtime.go`: project pods and events into workspace-visible objects.
- `pkg/workspace/projection/runtime_test.go`: projected pod identity and event projection tests.
- `pkg/workspace/live/resolve.go`: live target selection against workspace index state.
- `pkg/workspace/live/resolve_test.go`: unique-target and ambiguity tests.
- `pkg/registry/workspace/storage/storage.go`: storage factory for the workspace API group.
- `pkg/registry/workspace/storage/workspace.go`: `workspaces` REST storage.
- `pkg/registry/workspace/storage/workspace_test.go`: `workspaces` storage tests.
- `pkg/registry/workspace/storage/placementview.go`: read-only `placementviews` REST storage.
- `pkg/registry/workspace/storage/placementview_test.go`: `placementviews` tests.
- `pkg/registry/workspace/storage/errors.go`: explicit error constructors for unsupported and ambiguous requests.
- `pkg/registry/workspace/storage/errors_test.go`: error-shape tests.
- `pkg/registry/workspace/storage/pods.go`: projected `pods` read storage and pod live subresource plumbing.
- `pkg/registry/workspace/storage/pods_test.go`: pod storage tests.
- `pkg/registry/workspace/storage/events.go`: projected `events` read storage.
- `pkg/registry/workspace/storage/events_test.go`: event storage tests.
- `pkg/registry/workspace/storage/pod_live.go`: `logs`, `exec`, `attach`, and `portforward` connect handlers.
- `pkg/registry/workspace/storage/pod_live_test.go`: live subresource storage tests.
- `pkg/karmadactl/workspace/workspace.go`: parent `workspace` command.
- `pkg/karmadactl/workspace/kubeconfig.go`: `karmadactl workspace kubeconfig`.
- `pkg/karmadactl/workspace/kubeconfig_test.go`: kubeconfig generation tests.
- `test/e2e/framework/workspace.go`: workspace-specific test helpers for kubeconfig, endpoint checks, and object assertions.
- `test/e2e/suites/workspace/suite_test.go`: Ginkgo suite bootstrap for workspace e2e.
- `test/e2e/suites/workspace/workspace_api_test.go`: CRUD, discovery, and watch behavior tests through the workspace endpoint.
- `test/e2e/suites/workspace/workspace_live_test.go`: pod live-subresource and ambiguity tests through the workspace endpoint.
- `test/e2e/suites/workspace/README.md`: manual verification notes, including bounded `k9s` smoke validation.
- `artifacts/deploy/karmada-workspace-apiserver.yaml`: deployment, service, and RBAC for the new server.
- `artifacts/deploy/karmada-workspace-apiserver-apiservice.yaml`: aggregated APIService registration for `workspace.karmada.io`.

### Generated files and directories expected after codegen

- `pkg/apis/workspace/v1alpha1/zz_generated.deepcopy.go`
- `pkg/apis/workspace/v1alpha1/zz_generated.register.go`
- `pkg/generated/openapi/zz_generated.openapi.go`
- `pkg/generated/applyconfigurations/workspace/`
- `pkg/generated/clientset/versioned/typed/workspace/`
- `pkg/generated/informers/externalversions/workspace/`
- `pkg/generated/listers/workspace/`

### Decomposition rules

- Keep support-matrix decisions in one package and reuse them from storage, projection, live routing, and contract tests.
- Keep REST storage thin. Put translation, projection, and live-target logic under `pkg/workspace/*`.
- Compile writes into ordinary Karmada source objects. Do not introduce a parallel desired-state object model.
- Keep runtime projection separate from live routing. List/watch and `connect` semantics have different failure modes.
- Treat `kubectl` compatibility as an executable phase-1 requirement and `k9s` compatibility as a documented smoke bar.

## Tasks

### Task 1: Finalize Workspace API Types And Generation

**Files:**
- Modify: `pkg/apis/workspace/v1alpha1/types.go`
- Modify: `pkg/apis/workspace/v1alpha1/types_test.go`
- Modify: `pkg/apis/workspace/v1alpha1/register.go`
- Modify: `pkg/apis/workspace/install/install.go`
- Modify: `pkg/apis/workspace/scheme/register.go`
- Modify: `hack/update-codegen.sh`
- Modify: `pkg/apis/workspace/v1alpha1/zz_generated.deepcopy.go`
- Create: `pkg/apis/workspace/v1alpha1/zz_generated.register.go`
- Modify: `pkg/generated/openapi/zz_generated.openapi.go`
- Modify: `pkg/generated/applyconfigurations/workspace/`
- Modify: `pkg/generated/clientset/versioned/typed/workspace/`
- Modify: `pkg/generated/informers/externalversions/workspace/`
- Modify: `pkg/generated/listers/workspace/`

- [ ] **Step 1: Write the failing API contract tests**

```go
func TestWorkspacePublicContract(t *testing.T) {
	if LiveSubresourcePortForward != "portforward" {
		t.Fatalf("got %q", LiveSubresourcePortForward)
	}
	if ResourceKindWorkspace != "Workspace" {
		t.Fatalf("unexpected workspace kind: %s", ResourceKindWorkspace)
	}
	if ResourceKindPlacementView != "PlacementView" {
		t.Fatalf("unexpected placement view kind: %s", ResourceKindPlacementView)
	}
}

func TestNamespacePolicyModes(t *testing.T) {
	want := []NamespacePolicyMode{
		NamespacePolicyModeShared,
		NamespacePolicyModePrefixed,
		NamespacePolicyModeDedicated,
	}
	got := []NamespacePolicyMode{
		NamespacePolicyModeShared,
		NamespacePolicyModePrefixed,
		NamespacePolicyModeDedicated,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}
```

- [ ] **Step 2: Run the API tests to verify they fail**

Run: `go test ./pkg/apis/workspace/v1alpha1 -run 'TestWorkspacePublicContract|TestNamespacePolicyModes' -count=1`
Expected: FAIL because `PlacementView` and its metadata are not fully defined yet.

- [ ] **Step 3: Implement the phase-1 API types and add workspace codegen inputs**

```go
type PlacementView struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status PlacementViewStatus `json:"status,omitempty"`
}

type PlacementViewStatus struct {
	SubjectRef          ObjectReference    `json:"subjectRef,omitempty"`
	EligibleClusters    []string           `json:"eligibleClusters,omitempty"`
	SelectedClusters    []string           `json:"selectedClusters,omitempty"`
	AmbiguousLiveTarget bool               `json:"ambiguousLiveTarget,omitempty"`
	Conditions          []metav1.Condition `json:"conditions,omitempty"`
}
```

Add workspace API packages to every relevant section of `hack/update-codegen.sh`.

- [ ] **Step 4: Regenerate and verify generated artifacts**

Run: `bash hack/update-codegen.sh`
Expected: PASS and generate workspace artifacts under `pkg/generated/`.

Run: `go test ./pkg/apis/workspace/v1alpha1 -count=1`
Expected: PASS.

Run: `bash hack/verify-codegen.sh`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/apis/workspace/v1alpha1/types.go pkg/apis/workspace/v1alpha1/types_test.go pkg/apis/workspace/v1alpha1/register.go pkg/apis/workspace/install/install.go pkg/apis/workspace/scheme/register.go hack/update-codegen.sh pkg/apis/workspace/v1alpha1/zz_generated.deepcopy.go pkg/apis/workspace/v1alpha1/zz_generated.register.go pkg/generated/openapi/zz_generated.openapi.go pkg/generated/applyconfigurations/workspace pkg/generated/clientset/versioned/typed/workspace pkg/generated/informers/externalversions/workspace pkg/generated/listers/workspace
git commit -m "feat: finalize workspace api types"
```

### Task 2: Add The Workspace CLI Surface

**Files:**
- Create: `pkg/karmadactl/workspace/workspace.go`
- Create: `pkg/karmadactl/workspace/kubeconfig.go`
- Create: `pkg/karmadactl/workspace/kubeconfig_test.go`
- Modify: `pkg/karmadactl/karmadactl.go`

- [ ] **Step 1: Write the failing kubeconfig tests**

```go
func TestBuildWorkspaceKubeconfigUsesPublishedURL(t *testing.T) {
	cfg, err := buildWorkspaceKubeconfig(baseConfig, "team-a", "https://karmada.example.com/apis/workspace.karmada.io/v1alpha1/workspaces/team-a/proxy/")
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Clusters["workspace-team-a"].Server; got != "https://karmada.example.com/apis/workspace.karmada.io/v1alpha1/workspaces/team-a/proxy/" {
		t.Fatalf("got %s", got)
	}
}
```

- [ ] **Step 2: Run the workspace CLI tests to verify they fail**

Run: `go test ./pkg/karmadactl/workspace -run TestBuildWorkspaceKubeconfigUsesPublishedURL -count=1`
Expected: FAIL because the package does not exist yet.

- [ ] **Step 3: Implement the `workspace` command tree and kubeconfig command**

```go
func NewCmdWorkspace(f util.Factory, parentCommand string, streams genericiooptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Manage workspace access and configuration",
	}
	cmd.AddCommand(NewCmdKubeconfig(f, parentCommand, streams))
	return cmd
}
```

The command should read `Workspace.status.url` first and only fall back to the deterministic phase-1 path shape when status is empty in tests or local bootstrap scenarios.

- [ ] **Step 4: Run the workspace CLI tests**

Run: `go test ./pkg/karmadactl/workspace -count=1`
Expected: PASS.

Run: `go test ./pkg/karmadactl/... -run 'TestBuildWorkspaceKubeconfigUsesPublishedURL|TestNewKarmadaCtlCommand' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/karmadactl/workspace/workspace.go pkg/karmadactl/workspace/kubeconfig.go pkg/karmadactl/workspace/kubeconfig_test.go pkg/karmadactl/karmadactl.go
git commit -m "feat: add workspace cli commands"
```

### Task 3: Bootstrap The Workspace API Server

**Files:**
- Create: `cmd/karmada-workspace-apiserver/main.go`
- Create: `cmd/karmada-workspace-apiserver/app/karmada-workspace-apiserver.go`
- Create: `cmd/karmada-workspace-apiserver/app/options/options.go`
- Create: `cmd/karmada-workspace-apiserver/app/options/options_test.go`
- Create: `pkg/workspace/apiserver.go`
- Create: `pkg/workspace/apiserver_test.go`

- [ ] **Step 1: Write the failing server bootstrap tests**

```go
func TestWorkspaceAPIServerInstallsWorkspaceGroup(t *testing.T) {
	server, err := completedConfig.New()
	if err != nil {
		t.Fatal(err)
	}
	if server.GenericAPIServer == nil {
		t.Fatal("expected generic apiserver")
	}
}
```

- [ ] **Step 2: Run the server bootstrap tests to verify they fail**

Run: `go test ./pkg/workspace ./cmd/karmada-workspace-apiserver/... -run TestWorkspaceAPIServerInstallsWorkspaceGroup -count=1`
Expected: FAIL because the server packages do not exist yet.

- [ ] **Step 3: Implement the new binary and install the workspace API group**

```go
apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(
	workspaceapis.GroupName,
	workspacescheme.Scheme,
	workspacescheme.ParameterCodec,
	workspacescheme.Codecs,
)
apiGroupInfo.VersionedResourcesStorageMap["v1alpha1"] = storageMap
```

Mirror the shape of `cmd/karmada-search` and `pkg/search/apiserver.go` rather than inventing a new startup pattern.

- [ ] **Step 4: Run the bootstrap tests**

Run: `go test ./pkg/workspace ./cmd/karmada-workspace-apiserver/... -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/karmada-workspace-apiserver/main.go cmd/karmada-workspace-apiserver/app/karmada-workspace-apiserver.go cmd/karmada-workspace-apiserver/app/options/options.go cmd/karmada-workspace-apiserver/app/options/options_test.go pkg/workspace/apiserver.go pkg/workspace/apiserver_test.go
git commit -m "feat: bootstrap workspace apiserver"
```

### Task 4: Add Workspace Root Storage And Error Semantics

**Files:**
- Create: `pkg/registry/workspace/storage/storage.go`
- Create: `pkg/registry/workspace/storage/workspace.go`
- Create: `pkg/registry/workspace/storage/workspace_test.go`
- Create: `pkg/registry/workspace/storage/placementview.go`
- Create: `pkg/registry/workspace/storage/placementview_test.go`
- Create: `pkg/registry/workspace/storage/errors.go`
- Create: `pkg/registry/workspace/storage/errors_test.go`
- Modify: `pkg/workspace/apiserver.go`

- [ ] **Step 1: Write the failing storage and error tests**

```go
func TestStorageInstallsExpectedResources(t *testing.T) {
	storage, err := NewStorage(...)
	if err != nil {
		t.Fatal(err)
	}
	if storage.Workspaces == nil || storage.PlacementViews == nil {
		t.Fatal("expected workspace root storages")
	}
}

func TestNewAmbiguousTargetErrorUsesConflict(t *testing.T) {
	err := NewAmbiguousTargetError("pods", "demo")
	if !apierrors.IsConflict(err) {
		t.Fatalf("expected conflict, got %v", err)
	}
}
```

- [ ] **Step 2: Run the storage tests to verify they fail**

Run: `go test ./pkg/registry/workspace/storage -run 'TestStorageInstallsExpectedResources|TestNewAmbiguousTargetErrorUsesConflict' -count=1`
Expected: FAIL because the storage package does not exist yet.

- [ ] **Step 3: Implement thin REST storage and explicit errors**

```go
type Storage struct {
	Workspaces     rest.Storage
	PlacementViews rest.Storage
}
```

`errors.go` must provide stable helpers for:
- unsupported resource or verb
- ambiguous live target
- no eligible live target
- placement visibility denied

- [ ] **Step 4: Run the storage tests**

Run: `go test ./pkg/registry/workspace/storage -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/registry/workspace/storage/storage.go pkg/registry/workspace/storage/workspace.go pkg/registry/workspace/storage/workspace_test.go pkg/registry/workspace/storage/placementview.go pkg/registry/workspace/storage/placementview_test.go pkg/registry/workspace/storage/errors.go pkg/registry/workspace/storage/errors_test.go pkg/workspace/apiserver.go
git commit -m "feat: add workspace root storage"
```

### Task 5: Encode The Phase-1 Support Matrix And Conservative Query Planning

**Files:**
- Create: `pkg/workspace/support/matrix.go`
- Create: `pkg/workspace/support/matrix_test.go`
- Modify: `pkg/workspace/planner/planner.go`
- Modify: `pkg/workspace/planner/planner_test.go`
- Modify: `pkg/registry/workspace/storage/errors.go`

- [ ] **Step 1: Write the failing support-matrix and planner tests**

```go
func TestPhase1Matrix(t *testing.T) {
	if !Supports(deploymentsGVR, "create") {
		t.Fatal("expected deployment create support")
	}
	if Supports(eventsGVR, "create") {
		t.Fatal("did not expect event create support")
	}
}

func TestConservativePushdownRejectsUnsupportedFieldSelectors(t *testing.T) {
	plan, err := ConservativePushdown(QueryRequest{FieldSelector: "status.phase=Running"}, []string{"member1"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Supported() {
		t.Fatal("expected unsupported field selector")
	}
}
```

- [ ] **Step 2: Run the support and planner tests to verify they fail**

Run: `go test ./pkg/workspace/support ./pkg/workspace/planner -count=1`
Expected: FAIL because the support matrix package does not exist yet.

- [ ] **Step 3: Implement the matrix and route planner through it**

```go
type Capability struct {
	Read        bool
	Write       bool
	Watch       bool
	ConnectSubs sets.Set[string]
}
```

Use the matrix as the single source of truth for:
- discovery exposure
- write admission
- projected read-only behavior
- pod live subresource exposure

- [ ] **Step 4: Run the support and planner tests**

Run: `go test ./pkg/workspace/support ./pkg/workspace/planner -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/workspace/support/matrix.go pkg/workspace/support/matrix_test.go pkg/workspace/planner/planner.go pkg/workspace/planner/planner_test.go pkg/registry/workspace/storage/errors.go
git commit -m "feat: add workspace support matrix"
```

### Task 6: Compile Writable Workspace Requests Into Main-Karmada Source Objects

**Files:**
- Create: `pkg/workspace/facade/metadata.go`
- Create: `pkg/workspace/facade/sourceobjects.go`
- Create: `pkg/workspace/facade/sourceobjects_test.go`
- Modify: `pkg/registry/workspace/storage/workspace.go`
- Modify: `pkg/workspace/support/matrix.go`

- [ ] **Step 1: Write the failing desired-state translation tests**

```go
func TestCompileDeploymentWriteToSourceObject(t *testing.T) {
	obj, err := CompileDesiredState(deploymentsGVR, workspace, deployment)
	if err != nil {
		t.Fatal(err)
	}
	if obj.GetNamespace() != "team-a" {
		t.Fatalf("got namespace %s", obj.GetNamespace())
	}
	if obj.GetAnnotations()[AnnotationWorkspaceName] != "team-a" {
		t.Fatalf("missing workspace annotation")
	}
}
```

- [ ] **Step 2: Run the facade tests to verify they fail**

Run: `go test ./pkg/workspace/facade -run TestCompileDeploymentWriteToSourceObject -count=1`
Expected: FAIL because the facade package does not exist yet.

- [ ] **Step 3: Implement translation into ordinary source objects**

```go
func CompileDesiredState(gvr schema.GroupVersionResource, ws *workspacev1alpha1.Workspace, obj client.Object) (client.Object, error) {
	out := obj.DeepCopyObject().(client.Object)
	meta := out.GetAnnotations()
	if meta == nil {
		meta = map[string]string{}
	}
	meta[AnnotationWorkspaceName] = ws.Name
	out.SetAnnotations(meta)
	return out, nil
}
```

Hard requirements:
- write ordinary source objects into the main Karmada apiserver
- attach enough workspace metadata for reverse projection and debug
- do not create or mutate `Work` as the user-facing desired-state API

- [ ] **Step 4: Run the facade and storage tests**

Run: `go test ./pkg/workspace/facade ./pkg/registry/workspace/storage -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/workspace/facade/metadata.go pkg/workspace/facade/sourceobjects.go pkg/workspace/facade/sourceobjects_test.go pkg/registry/workspace/storage/workspace.go pkg/workspace/support/matrix.go
git commit -m "feat: compile workspace writes to source objects"
```

### Task 7: Project Pods And Events Into The Workspace View

**Files:**
- Create: `pkg/workspace/index/types.go`
- Create: `pkg/workspace/index/types_test.go`
- Create: `pkg/workspace/projection/runtime.go`
- Create: `pkg/workspace/projection/runtime_test.go`
- Create: `pkg/registry/workspace/storage/pods.go`
- Create: `pkg/registry/workspace/storage/pods_test.go`
- Create: `pkg/registry/workspace/storage/events.go`
- Create: `pkg/registry/workspace/storage/events_test.go`

- [ ] **Step 1: Write the failing projection tests**

```go
func TestProjectedPodNameIsDeterministicOnCollision(t *testing.T) {
	got := ProjectPodName("nginx-abc", []string{"member-b", "member-a"})
	if got == "nginx-abc" {
		t.Fatal("expected synthesized stable name on collision")
	}
}

func TestProjectedEventPreservesInvolvedObjectReference(t *testing.T) {
	event := ProjectEvent(inputEvent, projectedPodRef)
	if event.InvolvedObject.Name != projectedPodRef.Name {
		t.Fatalf("got %s want %s", event.InvolvedObject.Name, projectedPodRef.Name)
	}
}
```

- [ ] **Step 2: Run the projection tests to verify they fail**

Run: `go test ./pkg/workspace/index ./pkg/workspace/projection ./pkg/registry/workspace/storage -run 'TestProjectedPodNameIsDeterministicOnCollision|TestProjectedEventPreservesInvolvedObjectReference' -count=1`
Expected: FAIL because projection packages do not exist yet.

- [ ] **Step 3: Implement workspace projection and read-only storage**

```go
func ProjectPodName(name string, clusterSet []string) string {
	if len(clusterSet) <= 1 {
		return name
	}
	return fmt.Sprintf("%s-%s", name, stableSuffix(clusterSet))
}
```

Requirements:
- pods and events are projected read-only resources
- projected pod names are deterministic and DNS-label-safe
- raw cluster names do not become primary object identity
- list/watch read from workspace index snapshots, not direct fan-out

- [ ] **Step 4: Run the projection and storage tests**

Run: `go test ./pkg/workspace/index ./pkg/workspace/projection ./pkg/registry/workspace/storage -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/workspace/index/types.go pkg/workspace/index/types_test.go pkg/workspace/projection/runtime.go pkg/workspace/projection/runtime_test.go pkg/registry/workspace/storage/pods.go pkg/registry/workspace/storage/pods_test.go pkg/registry/workspace/storage/events.go pkg/registry/workspace/storage/events_test.go
git commit -m "feat: add projected workspace runtime resources"
```

### Task 8: Add Live Pod Subresource Routing

**Files:**
- Create: `pkg/workspace/live/resolve.go`
- Create: `pkg/workspace/live/resolve_test.go`
- Create: `pkg/registry/workspace/storage/pod_live.go`
- Create: `pkg/registry/workspace/storage/pod_live_test.go`
- Modify: `pkg/registry/workspace/storage/pods.go`
- Modify: `pkg/registry/workspace/storage/errors.go`

- [ ] **Step 1: Write the failing live-routing tests**

```go
func TestResolveUniqueLiveTarget(t *testing.T) {
	target, err := ResolveLiveTarget(indexState, request)
	if err != nil {
		t.Fatal(err)
	}
	if target.Cluster != "member-a" {
		t.Fatalf("got %s", target.Cluster)
	}
}

func TestResolveLiveTargetReturnsConflictOnAmbiguity(t *testing.T) {
	_, err := ResolveLiveTarget(indexState, request)
	if !apierrors.IsConflict(err) {
		t.Fatalf("expected conflict, got %v", err)
	}
}
```

- [ ] **Step 2: Run the live-routing tests to verify they fail**

Run: `go test ./pkg/workspace/live ./pkg/registry/workspace/storage -run 'TestResolveUniqueLiveTarget|TestResolveLiveTargetReturnsConflictOnAmbiguity' -count=1`
Expected: FAIL because the live package does not exist yet.

- [ ] **Step 3: Implement live target resolution and `connect` storage**

```go
func ResolveLiveTarget(index *State, req Request) (Target, error) {
	candidates := eligibleTargets(index, req)
	switch len(candidates) {
	case 0:
		return Target{}, NewNoEligibleTargetError(req.Resource, req.Name)
	case 1:
		return candidates[0], nil
	default:
		return Target{}, NewAmbiguousTargetError(req.Resource, req.Name)
	}
}
```

Support only:
- `logs`
- `exec`
- `attach`
- `portforward`

Do not add generic `proxy` support in phase 1.

- [ ] **Step 4: Run the live-routing tests**

Run: `go test ./pkg/workspace/live ./pkg/registry/workspace/storage -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/workspace/live/resolve.go pkg/workspace/live/resolve_test.go pkg/registry/workspace/storage/pod_live.go pkg/registry/workspace/storage/pod_live_test.go pkg/registry/workspace/storage/pods.go pkg/registry/workspace/storage/errors.go
git commit -m "feat: add workspace live pod routing"
```

### Task 9: Wire Deployment Artifacts

**Files:**
- Create: `artifacts/deploy/karmada-workspace-apiserver.yaml`
- Create: `artifacts/deploy/karmada-workspace-apiserver-apiservice.yaml`
- Modify: `docs/superpowers/specs/2026-03-21-workspace-giant-cluster-design.md`

- [ ] **Step 1: Write the failing deploy-shape checks**

```bash
rg -n "karmada-workspace-apiserver" artifacts/deploy
```

Expected: no matches before the manifests are added.

- [ ] **Step 2: Create deploy manifests**

Include:
- Deployment
- Service
- ServiceAccount
- ClusterRole and ClusterRoleBinding
- APIService for `v1alpha1.workspace.karmada.io`

Keep the manifest minimal and phase-1 scoped.

- [ ] **Step 3: Verify manifests are present**

Run: `rg -n "workspace.karmada.io|karmada-workspace-apiserver" artifacts/deploy docs/superpowers/specs/2026-03-21-workspace-giant-cluster-design.md`
Expected: PASS with the new manifests and updated spec references.

- [ ] **Step 4: Commit**

```bash
git add artifacts/deploy/karmada-workspace-apiserver.yaml artifacts/deploy/karmada-workspace-apiserver-apiservice.yaml docs/superpowers/specs/2026-03-21-workspace-giant-cluster-design.md
git commit -m "feat: wire workspace apiserver deployment"
```

### Task 10: Add Workspace API Contract Tests

**Files:**
- Create: `pkg/workspace/apiserver_contract_test.go`
- Modify: `pkg/workspace/apiserver.go`
- Modify: `pkg/workspace/support/matrix.go`
- Modify: `pkg/registry/workspace/storage/errors.go`

- [ ] **Step 1: Write the failing contract tests**

```go
func TestWorkspaceDiscoveryOnlyAdvertisesSupportedResources(t *testing.T) {
	server := newTestWorkspaceServer(t)
	resources := discoverResources(t, server)
	if containsResource(resources, "nodes") {
		t.Fatal("did not expect nodes in phase-1 discovery")
	}
	if !containsResource(resources, "pods") {
		t.Fatal("expected pods in phase-1 discovery")
	}
}

func TestWorkspacePodLiveAmbiguityReturnsConflict(t *testing.T) {
	server := newTestWorkspaceServer(t)
	resp := connectPodSubresource(t, server, "logs", ambiguousPodRequest())
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("got %d", resp.StatusCode)
	}
}
```

- [ ] **Step 2: Run the contract tests to verify they fail**

Run: `go test ./pkg/workspace -run 'TestWorkspaceDiscoveryOnlyAdvertisesSupportedResources|TestWorkspacePodLiveAmbiguityReturnsConflict' -count=1`
Expected: FAIL because the contract-test scaffolding does not exist yet.

- [ ] **Step 3: Implement black-box contract tests around the workspace server**

```go
func newTestWorkspaceServer(t *testing.T) *httptest.Server {
	t.Helper()
	server, err := completedConfigForTest(t).New()
	if err != nil {
		t.Fatal(err)
	}
	return httptest.NewServer(server.GenericAPIServer.Handler.NonGoRestfulMux)
}
```

The contract tests must verify:
- truthful discovery
- `404` for unsupported resources and subresources
- `405` for unsupported verbs on discovered resources
- `403` passthrough shape for authorization failures
- `404` and `409` live-target errors
- watch availability only on supported resources

- [ ] **Step 4: Run the contract tests**

Run: `go test ./pkg/workspace -run 'TestWorkspaceDiscoveryOnlyAdvertisesSupportedResources|TestWorkspacePodLiveAmbiguityReturnsConflict|TestWorkspaceWatchContract|TestWorkspaceUnsupportedVerbContract' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/workspace/apiserver_contract_test.go pkg/workspace/apiserver.go pkg/workspace/support/matrix.go pkg/registry/workspace/storage/errors.go
git commit -m "test: add workspace api contract coverage"
```

### Task 11: Add Workspace E2E And Client Compatibility Suite

**Files:**
- Create: `test/e2e/framework/workspace.go`
- Create: `test/e2e/suites/workspace/suite_test.go`
- Create: `test/e2e/suites/workspace/workspace_api_test.go`
- Create: `test/e2e/suites/workspace/workspace_live_test.go`
- Create: `test/e2e/suites/workspace/README.md`

- [ ] **Step 1: Write the failing workspace e2e skeleton and helper tests**

```go
var _ = ginkgo.Describe("Workspace API", ginkgo.Ordered, func() {
	ginkgo.It("supports kubectl CRUD on writable resources", func() {
		output, err := framework.RunWorkspaceKubectl(workspaceKubeconfig, "get", "configmaps", "-n", testNamespace)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("NAME"))
	})
})
```

- [ ] **Step 2: Run the workspace e2e package to verify it fails**

Run: `go test ./test/e2e/suites/workspace -count=1`
Expected: FAIL because the suite and helpers do not exist yet.

- [ ] **Step 3: Implement the workspace suite and kubectl-driven checks**

```go
func RunWorkspaceKubectl(kubeconfigPath string, args ...string) (string, error) {
	cmd := exec.Command("kubectl", append([]string{"--kubeconfig", kubeconfigPath}, args...)...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
```

The suite must cover:
- generating workspace kubeconfig through `karmadactl workspace kubeconfig`
- `kubectl api-resources` only showing the supported phase-1 surface
- CRUD for supported writable resources
- `kubectl get pods`, `kubectl get events`, and `kubectl get -w` on supported resources
- `kubectl logs`, `kubectl exec`, `kubectl attach`, and `kubectl port-forward` on uniquely resolved pod targets
- `409 Conflict` behavior for ambiguous live targets

Document `k9s` as a manual smoke checklist in `test/e2e/suites/workspace/README.md` instead of pretending it is stable CI automation in phase 1.

- [ ] **Step 4: Run the workspace e2e suite**

Run: `go test ./test/e2e/suites/workspace -count=1`
Expected: PASS for suite bootstrap and helper compilation.

Run: `ginkgo -v ./test/e2e/suites/workspace -- --poll-interval=5s --poll-timeout=5m`
Expected: PASS against a phase-1 workspace-capable environment.

- [ ] **Step 5: Commit**

```bash
git add test/e2e/framework/workspace.go test/e2e/suites/workspace/suite_test.go test/e2e/suites/workspace/workspace_api_test.go test/e2e/suites/workspace/workspace_live_test.go test/e2e/suites/workspace/README.md
git commit -m "test: add workspace e2e compatibility suite"
```

## Behavior Verification

The phase-1 execution path should include a behavioral verification layer in addition to package tests.

Required verification bars:

- `kubectl` is a required compatibility bar for the supported phase-1 surface;
- `k9s` is a bounded best-effort compatibility bar for the supported workload surface;
- failures caused by dishonest discovery or advertised-but-broken semantics are release blockers;
- unsupported resources are acceptable only when they are omitted from discovery and fail cleanly when addressed directly.

Recommended harness layering:

- package and storage tests for API, planner, projection, and live routing;
- apiserver contract tests for discovery, verbs, status codes, and watch behavior;
- Karmada e2e tests under `test/e2e/suites/workspace/` using the existing Ginkgo/Gomega harness;
- later optional Sonobuoy packaging for portable black-box execution once the workspace suite is stable.

Minimum CLI behavior checks to add during phase-1 execution:

- `kubectl api-resources`, `kubectl get`, `kubectl describe`, `kubectl create`, `kubectl apply`, and `kubectl delete` for supported writable resources;
- `kubectl get pods`, `kubectl logs`, `kubectl exec`, `kubectl attach`, and `kubectl port-forward` for projected pod flows;
- `kubectl get events` and `kubectl get -w` on supported resources;
- `k9s` smoke validation for namespace, workload, pod, event, log, and shell navigation against the workspace kubeconfig.

## Verification Checklist

Run these before claiming the plan is fully executed:

- `go test ./pkg/apis/workspace/v1alpha1 -count=1`
- `go test ./pkg/workspace/... -count=1`
- `go test ./pkg/registry/workspace/storage -count=1`
- `go test ./pkg/karmadactl/workspace -count=1`
- `go test ./cmd/karmada-workspace-apiserver/... -count=1`
- `go test ./test/e2e/suites/workspace -count=1`
- `ginkgo -v ./test/e2e/suites/workspace -- --poll-interval=5s --poll-timeout=5m`
- `bash hack/verify-codegen.sh`

## Non-Goals For This Plan

- Cluster-scoped policy or admin APIs in the workspace surface
- Arbitrary CRD passthrough
- Generic per-request member-cluster selection UX
- Direct desired-state writes to member clusters
- Replacing existing `clusters/*/proxy` operator paths
- Full `k9s` parity across unsupported cluster-scoped or metrics-backed views in phase 1
