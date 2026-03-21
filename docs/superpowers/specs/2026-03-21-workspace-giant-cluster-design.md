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

