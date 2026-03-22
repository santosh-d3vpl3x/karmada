# Workspace E2E

This suite exercises the workspace compatibility surface through a generated workspace kubeconfig created by `karmadactl workspace kubeconfig`.

## Automated Coverage

The suite is intentionally honest about environmental readiness.

- `go test ./test/e2e/suites/workspace -count=1` verifies the suite compiles and skips cleanly when the current cluster does not expose a workspace-capable endpoint.
- `ginkgo -v ./test/e2e/suites/workspace -- --poll-interval=5s --poll-timeout=5m` runs the kubectl compatibility checks when the environment can actually serve the workspace surface exercised by this branch.

The automated suite covers the currently implemented workspace surface on this branch:

- workspace kubeconfig generation through `karmadactl workspace kubeconfig`
- `kubectl api-resources` over the supported workspace surface
- namespace creation and visibility through the workspace API
- CRUD-oriented flows for the writable desired-state resources
- `kubectl get pods`
- `kubectl get events`
- `kubectl get -w` on a supported resource
- `kubectl logs`
- `kubectl exec`
- `kubectl attach`
- `kubectl port-forward`
- `409 Conflict` behavior for ambiguous live targets when `WORKSPACE_E2E_AMBIGUOUS_POD` names a known ambiguous workspace-visible pod
- live-target inspection through `?inspect=1` for an intentionally ambiguous pod
- explicit live-target selection through `?targetCluster=<member>` for an intentionally ambiguous pod
- `PlacementView` read-only debug output for a workspace-visible pod through the control-plane workspace API group

## Environment Requirements

The suite assumes all of the following are true:

- `KUBECONFIG` points at a Karmada control-plane kubeconfig
- `$(go env GOPATH)/bin/karmadactl` exists
- the target environment publishes the workspace proxy endpoint and truthful workspace discovery
- desired-state writes for supported namespaced resources, projected runtime reads, and live pod subresources are wired in the deployed workspace apiserver
- the control-plane kubeconfig can reach `placementviews.workspace.karmada.io` read endpoints when the phase-2 debug surface is deployed
- `WORKSPACE_E2E_AMBIGUOUS_POD` names a known ambiguous workspace-visible pod when running the ambiguity inspection and explicit-target-selection checks

If those prerequisites are not met, the suite skips instead of pretending the current workspace surface works.

## Manual `k9s` Smoke Checklist

`k9s` is a manual smoke bar only in phase 1. Do not treat the following as CI automation.

1. Generate a workspace kubeconfig with `karmadactl workspace kubeconfig <workspace> > /tmp/<workspace>.kubeconfig`.
2. Launch `k9s --kubeconfig /tmp/<workspace>.kubeconfig`.
3. Confirm the default views show logical workspace namespaces, workloads, pods, and events without exposing unsupported cluster-scoped surfaces such as nodes.
4. Navigate from a workload to a pod and verify log viewing works on the supported workload surface.
5. Verify shell and port-forward actions behave normally for a uniquely resolved pod target.
6. If an intentionally ambiguous live target is available, confirm the operation fails explicitly instead of silently selecting a backing cluster.
