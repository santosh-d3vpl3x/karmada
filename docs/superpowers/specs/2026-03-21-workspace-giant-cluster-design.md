---
title: Workspace Giant-Cluster Design
date: 2026-03-21
status: draft
related:
  - ../../proposals/virtual-cluster-workspace/README.md
---

# Workspace Giant-Cluster Design

## Summary

Karmada should expose a tenant-facing `Workspace` API that makes multiple backing Kubernetes clusters feel like one logical Kubernetes cluster.

The user connects to one workspace endpoint and uses standard Kubernetes clients and workflows. Normal CRUD is authoritative in Karmada control-plane state. Live subresources such as `logs`, `exec`, `attach`, and `port-forward` are resolved to exactly one eligible backing target and proxied through with user impersonation.

This design does not extend `clusters/*/proxy` into a tenant API. That path remains useful for operator-oriented fan-out access, but it is not a stable foundation for one-cluster semantics.

## Design Principles

### Preserve Kubernetes intuition

The workspace API should behave like Kubernetes wherever possible. Divergence must be explicit, minimal, and justified.

Implications:

- one workspace endpoint should feel like one kube-apiserver;
- user-facing names should stay Kubernetes-like;
- supported resources must have truthful discovery and real Kubernetes-style semantics;
- the API must not guess hidden routing decisions;
- asynchronous reconciliation should be surfaced the way Kubernetes surfaces it: through status, conditions, events, and observable object state.

### Stable architecture, phased surface area

Scope can tighten by phase, but deferred work must be recorded explicitly as later phases rather than silently dropped.

The implementation should build a generic workspace framework early, then enable more resources and behaviors incrementally on top of that framework.

## Why This Approach

### Rejected approach: extend wildcard proxy

Using `clusters/*/proxy` as the main tenant API is attractive because it already exists, but its semantics are wrong for the target experience:

- ambiguity is inherent to the path;
- identity leaks backing-cluster reality;
- `list/watch` semantics are not equivalent to one logical cluster;
- the model remains operator-centric.

### Rejected approach: expose search/cache directly

Cached or search-oriented proxy APIs are useful for inspection, but they are the wrong primary abstraction for end users:

- they tend to become admin views rather than true kube-apiserver surfaces;
- they create RBAC and freshness concerns;
- they are weak foundations for full CRUD, watch, and live subresources.

### Recommended approach: workspace virtual apiserver

Add a workspace-focused virtual apiserver backed by Karmada state, placement metadata, and selected runtime feeds. This is the strongest path to low-rework incremental delivery because the core model remains stable while resource enablement expands by phase.

## Core Contract

### Tenant boundary

`Workspace` is the user-facing tenant boundary. Each workspace has:

- one API endpoint;
- one generated kubeconfig entrypoint;
- one logical Kubernetes cluster view;
- one workspace-level authorization boundary.

### Object identity

Primary object identity must not expose backing-cluster names.

- User-facing names remain Kubernetes-like.
- Logical UIDs are defined by workspace/Karmada state, not by backing-cluster copy identity.
- Placement and backing-cluster details are secondary metadata, not primary identity.

### Namespace model

Namespaces are first-class logical Kubernetes objects in the workspace.

Phase 1 may realize them partially underneath, but the user contract is stable from day one: namespace workflows should feel normal.

### Read model

Normal reads come from a workspace index, not from ad hoc direct fan-out on every request.

This index is the default source for:

- `get`
- `list`
- `watch`

Direct cluster contact is reserved for:

- live subresources;
- narrowly defined freshness paths;
- internal reconciliation and runtime collection.

### Write model

Standard resource mutation is authoritative in Karmada/workspace storage.

- `create`, `update`, `patch`, and `delete` act on logical workspace objects;
- success means the control plane accepted desired state;
- backing-cluster realization is asynchronous;
- status, conditions, and events expose propagation progress.

The workspace API is not a synchronous multi-cluster transaction API.

### Delete model

Deletes should follow normal Kubernetes deletion behavior:

- logical objects remain visible with deletion state;
- finalization and cleanup remain observable;
- removal happens after reconciliation completes.

### Live subresource model

Live subresources are proxied to backing clusters only when exactly one eligible target exists.

Phase 1 covers:

- `logs`
- `exec`
- `attach`
- `port-forward`

Rules:

- auto-resolve when exactly one eligible target exists;
- fail explicitly on ambiguity;
- never guess;
- preserve authenticated user identity through impersonation.

### Authorization model

Authorization is layered:

- workspace RBAC controls entry to and use of the workspace API;
- live pass-through actions also enforce impersonated backing-cluster RBAC.

This keeps the workspace as a real tenant boundary without weakening backing-cluster authorization for requests that execute there.

### Placement visibility

Backing-cluster placement must not leak into primary object names or the default happy path.

Instead, placement is exposed through secondary surfaces such as:

- `status`
- conditions
- events
- dedicated debug or inspection APIs/commands

## Runtime Object Semantics

### Pods

For supported workloads, the workspace should expose real pod instances, not a fully collapsed synthetic workload-only runtime view.

That is more intuitive for Kubernetes users because:

- `kubectl get pods` still means real runtime instances;
- logs and exec remain meaningful;
- readiness and lifecycle remain familiar.

However, pod identity still needs to stay workspace-stable and Kubernetes-like. Raw per-cluster naming must not become the primary UX.

### Ambiguity handling

When the workspace cannot map an operation to exactly one valid backing target, it must return an explicit ambiguity error.

Later phases may add explicit target selection, but the system should never hide ambiguity behind heuristics.

## Discovery And Client Compatibility

Supported resources must be safe for standard Kubernetes-style clients and automation, not just humans with `kubectl`.

Therefore:

- discovery only advertises resources and subresources whose semantics are actually supported in that phase;
- phase 1 resources must support real workspace-level `list/watch` behavior;
- resource coverage may phase in, but semantics must not be downgraded for enabled resources.


## API And Data Model

### Workspace endpoint contract

The canonical workspace entrypoint is the published URL in `Workspace.status.url`.

Phase 1 should implement this using a path-based endpoint under the Karmada front door, for example:

`/apis/workspace.karmada.io/v1alpha1/workspaces/<name>/proxy/`

The kubeconfig generator should bind clients to the published URL rather than to a hardcoded transport shape. That keeps the contract stable if later phases move a workspace to a dedicated hostname.

### Workspace resource shape

The `Workspace` API should stay small but extensible. It should fix the tenant contract without introducing a large policy matrix too early.

Recommended structure:

- `spec.clusterSelector`: identifies the member clusters that may back the workspace;
- `spec.namespacePolicy`: controls which namespaces exist in the workspace view;
- `spec.liveAccess`: enables bounded live subresources such as `logs`, `exec`, `attach`, and `port-forward`;
- `spec.placementVisibility`: controls whether placement is hidden, summarized, or exposed through secondary surfaces;
- `spec.apiProfile`: declares the supported API envelope for the workspace;
- `status.url`: publishes the canonical endpoint;
- `status.phase`: exposes high-level readiness;
- `status.conditions`: publishes detailed readiness and degradation state;
- `status.observedGeneration`: tracks controller convergence;
- `status.resolvedClusters`: publishes the currently selected cluster set for debugging and administration;
- `status.effectiveAPIProfile`: publishes the realized profile after admission or defaulting.

### API profiles

Phased scope should be explicit in the API, not only in documents.

The design should use coarse profiles such as `workload-v1` rather than per-resource toggle sprawl. Discovery remains truthful at runtime, but `spec.apiProfile` makes the intended support envelope visible and auditable.

### Namespace model

Namespaces are first-class logical Kubernetes objects in the workspace.

In phase 1:

- namespace names remain stable and are never cluster-qualified;
- namespace CRUD is served by the workspace API;
- backing-cluster namespace realization may remain asynchronous and partial;
- namespace readiness or propagation state should be reflected through normal status and conditions.

### Object identity

The object identity model should preserve Kubernetes intuition while handling multi-cluster collisions safely.

Rules:

- Karmada-authoritative logical resources keep normal Kubernetes names;
- logical UIDs come from workspace or Karmada state, not from backing-cluster copy identity;
- runtime objects such as pods should keep their familiar names when those names are unique in the workspace view;
- if collisions occur, the workspace should mint a stable opaque suffix that is safe for Kubernetes names and does not encode cluster identity;
- backing-cluster names must not appear in primary user-facing object names.

### Resource version and watch model

Supported resources require workspace-scoped logical `resourceVersion` values and workspace-native watch semantics.

The design should not expose ad hoc merged per-cluster resourceVersion tokens as the long-term contract. Instead, the workspace layer should allocate and serve logical versions from its own index and event pipeline.

If a resource cannot yet satisfy that level of semantics, it should not be advertised as supported in that phase.

### Placement and debug model

Placement details should not be baked into primary object fields.

Instead, phase 1 should use secondary surfaces:

- conditions;
- events;
- summarized status where appropriate;
- dedicated debug or inspection APIs.

One useful secondary object is a read-only placement view for each logical resource. That view can expose:

- the logical object reference;
- selected backing clusters;
- realization state by backing cluster;
- ambiguity state for live-target resolution;
- health summary.

### Live target resolution model

Live target resolution should remain a runtime decision driven by workspace index state rather than a field baked into primary objects.

Rules:

- if exactly one eligible target exists, proxy the request there;
- if zero or multiple eligible targets exist, fail explicitly;
- secondary debug surfaces should explain why a live request was or was not uniquely resolvable.

### Phase-1 data model boundary

Phase 1 should keep the data model intentionally bounded:

- namespaced workload-centric resources;
- namespaces as logical workspace objects;
- pods and events as runtime projections;
- no requirement to solve every cluster-scoped or extension API up front.

Later phases should extend this boundary explicitly rather than changing the meaning of phase-1 contracts.

## Architecture

### Workspace apiserver

Introduce a `karmada-workspace-apiserver` that serves a workspace-scoped Kubernetes-compatible API surface.

Responsibilities:

- authentication using existing Karmada front-door auth;
- workspace authorization;
- logical REST storage;
- discovery for enabled resources;
- workspace-scoped watch streams;
- live subresource routing.

### Workspace index

Introduce a workspace index that materializes the logical cluster view.

The index should combine:

- Karmada source objects;
- placement state such as `ResourceBinding`, `ClusterResourceBinding`, and `Work`;
- runtime feeds for resources such as pods and events;
- logical identity and candidate target mapping.

This index must remain internal control-plane state, not a user-exposed authorization bypass cache.

### Query planner

Build a generic query planner underneath the workspace API from day one.

The planner should:

- answer supported requests from logical workspace state when possible;
- push down safe filters where useful;
- preserve final merge, authz, pagination, and logical identity semantics in the workspace layer.


### Workspace topology resolver

The workspace layer needs a controller that resolves `Workspace.spec` into effective runtime topology.

It should reconcile:

- cluster selection into an effective backing-cluster set;
- API profile into enabled resource and subresource exposure;
- published endpoint state into `status.url` and readiness conditions;
- workspace health into summarized status.

This is not the same as serving requests. It is the control loop that keeps the workspace contract current as cluster membership, policy, or configuration changes.

### Logical object storage

The workspace API should persist logical object state in Karmada-owned storage, not treat member clusters as the write-time source of truth.

For supported phase-1 resources, the storage layer should hold:

- logical metadata and UID;
- desired spec;
- deletion and finalization state;
- workspace-scoped status envelope;
- mapping references needed for propagation and runtime projection.

### Projection and index pipelines

The workspace index should be built by dedicated materialization pipelines rather than by request-time fan-out.

Recommended pipelines:

- source-object pipeline: consumes logical object changes from Karmada storage;
- placement pipeline: consumes `ResourceBinding`, `ClusterResourceBinding`, and `Work` state;
- runtime pipeline: consumes pod, event, and similar runtime feeds from backing clusters or pull-agent reports;
- watch pipeline: turns accepted logical changes and runtime projection updates into workspace-scoped watch events.

These pipelines feed the workspace index and the workspace logical `resourceVersion` stream.

### Reconciliation model

The design should use a small set of explicit reconcilers with clear ownership.

#### 1. Workspace reconciler

Reconciles `Workspace` objects.

What it reconciles:

- desired workspace spec;
- selected member clusters;
- effective API profile;
- endpoint publication;
- workspace conditions and readiness.

Outputs:

- `Workspace.status`;
- effective cluster membership for the workspace;
- enablement state for discovery and routing;
- controller subscriptions or watches required for that workspace.

#### 2. Namespace reconciler

Reconciles logical workspace namespaces.

What it reconciles:

- logical namespace existence in the workspace;
- backing namespace realization where phase-1 semantics require it;
- namespace termination and cleanup;
- namespace status and readiness.

Outputs:

- logical namespace object state;
- backing realization state;
- conditions and events that explain propagation progress.

#### 3. Logical resource reconciler

Reconciles supported logical resources whose source of truth lives in Karmada.

What it reconciles:

- accepted workspace CRUD into Karmada-authoritative desired state;
- propagation intent toward backing clusters;
- deletion and finalization state;
- high-level status reflected back into the logical resource.

This is the control loop that keeps `Deployment`, `Service`, `ConfigMap`, and similar supported resources aligned with Karmada propagation machinery.

#### 4. Runtime projection reconciler

Reconciles runtime observations into logical workspace runtime views.

What it reconciles:

- backing pod and event observations;
- placement and owner relationships;
- logical pod identity and collision handling;
- projected runtime status for workspace reads and watches.

This is how the workspace exposes runtime objects without making request-time fan-out the core data path.

#### 5. Placement and health reconciler

Reconciles secondary debug and placement views.

What it reconciles:

- logical object references;
- per-cluster realization state;
- ambiguity state for live routing;
- summarized health, drift, and propagation failures.

Outputs:

- secondary placement or debug objects;
- conditions and events used by operators and higher-level tooling.

### What is being reconciled exactly

The system is reconciling five different things, not one monolithic "giant cluster" object:

- workspace contract: which clusters, namespaces, and API profile define the tenant view;
- logical desired state: what the user asked the workspace to create, update, or delete;
- backing realization state: how that desired state is actually landing across member clusters;
- runtime projection state: what pods, events, and similar runtime objects should appear in the workspace view;
- debug and health state: what the system should report about placement, ambiguity, and convergence.

That split matters because each of those state domains changes at a different rate and has different correctness requirements.

### What is not reconciled

Some behaviors should stay request-driven rather than controller-driven.

These are not long-running reconciliation targets:

- `logs`;
- `exec`;
- `attach`;
- `port-forward`;
- one-off live target resolution.

Those operations are resolved at request time against the current workspace index. The reconciled part is the index and placement knowledge they depend on, not the live session itself.

## Phases

### Phase 1: usable tenant workspace

Deliver the stable framework and a bounded, namespaced, workload-centric surface.

Goals:

- workspace API and kubeconfig entrypoint;
- workspace-level namespace model;
- truthful discovery for enabled resources;
- Kubernetes-like `get/list/watch` semantics for supported resources;
- Karmada-authoritative CRUD for supported logical resources;
- live pass-through for `logs`, `exec`, `attach`, and `port-forward`;
- explicit ambiguity errors;
- secondary placement and debug visibility;
- support for standard clients and automation on the enabled surface.

Initial resource scope should stay bounded, for example:

- namespaces;
- configmaps;
- secrets;
- services;
- deployments;
- statefulsets;
- daemonsets;
- jobs;
- cronjobs;
- pods;
- events.

### Phase 2: broader coverage and explicit debug affordances

Expand the surface without changing the core model.

Examples:

- additional built-in namespaced resources;
- selected cluster-scoped views with clear logical semantics;
- explicit target-selection or inspection support for ambiguous runtime operations;
- stronger placement and runtime debugging surfaces.

### Phase 3: broader propagated-resource and extension support

Handle the harder cases that need the same framework but more semantic work.

Examples:

- broader cluster-scoped resources;
- CRDs and arbitrary propagated resources where semantics are well-defined;
- more complete extension and aggregated API behavior;
- deeper fidelity improvements where phase 1 and 2 intentionally stayed bounded.

## Deferred Work That Must Stay Explicit

The following items are intentionally not phase-1 requirements, but they remain part of the roadmap rather than being dropped:

- broader built-in API coverage;
- richer cluster-scoped virtualization;
- explicit target selection for ambiguous live operations;
- CRD and arbitrary propagated-resource support;
- harder aggregated API and extension semantics;
- deeper runtime fidelity and debug ergonomics.

## Recommendation

Proceed with a workspace virtual apiserver design, not a tenantized wildcard proxy.

Design the core once:

- workspace boundary;
- logical identity;
- workspace index;
- query planner;
- truthful discovery;
- Kubernetes-like semantics;
- layered authorization;
- explicit phased expansion.

Then ship incrementally by enabling resources and behaviors in phases without changing the contract.
