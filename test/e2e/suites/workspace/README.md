# Workspace E2E

This suite exercises the phase-1 workspace compatibility surface through a generated workspace kubeconfig created by `karmadactl workspace kubeconfig`.

## Automated Coverage

The suite is intentionally honest about environmental readiness.

- `go test ./test/e2e/suites/workspace -count=1` verifies the suite compiles and skips cleanly when the current cluster does not expose a workspace-capable endpoint.
- `ginkgo -v ./test/e2e/suites/workspace -- --poll-interval=5s --poll-timeout=5m` runs the kubectl compatibility checks when the environment can actually serve the phase-1 workspace surface.

The automated suite covers:

- workspace kubeconfig generation through `karmadactl workspace kubeconfig`
- `kubectl api-resources` over the supported phase-1 surface only
- CRUD-oriented flows for the writable phase-1 resources
- `kubectl get pods`
- `kubectl get events`
- `kubectl get -w` on a supported resource
- `kubectl logs`
- `kubectl exec`
- `kubectl attach`
- `kubectl port-forward`
- `409 Conflict` behavior for ambiguous live targets when `WORKSPACE_E2E_AMBIGUOUS_POD` names a known ambiguous workspace-visible pod

## Environment Requirements

The suite assumes all of the following are true:

- `KUBECONFIG` points at a Karmada control-plane kubeconfig
- `$(go env GOPATH)/bin/karmadactl` exists
- the target environment publishes the workspace proxy endpoint and truthful phase-1 discovery
- desired-state writes, projected runtime reads, and live pod subresources are wired in the deployed workspace apiserver

If those prerequisites are not met, the suite skips instead of pretending the phase-1 surface works.

## Manual `k9s` Smoke Checklist

`k9s` is a manual smoke bar only in phase 1. Do not treat the following as CI automation.

1. Generate a workspace kubeconfig with `karmadactl workspace kubeconfig <workspace> > /tmp/<workspace>.kubeconfig`.
2. Launch `k9s --kubeconfig /tmp/<workspace>.kubeconfig`.
3. Confirm the default views show namespaces, workloads, pods, and events without exposing unsupported cluster-scoped surfaces such as nodes.
4. Navigate from a workload to a pod and verify log viewing works on the supported workload surface.
5. Verify shell and port-forward actions behave normally for a uniquely resolved pod target.
6. If an intentionally ambiguous live target is available, confirm the operation fails explicitly instead of silently selecting a backing cluster.
