# Workspace E2E

This suite exercises the workspace compatibility surface through a generated workspace kubeconfig created by `karmadactl workspace kubeconfig`.

## Automated Coverage

The suite is intentionally honest about environmental readiness.

- `go test ./test/e2e/suites/workspace -count=1` verifies the suite compiles and skips cleanly when the current cluster does not expose a workspace-capable endpoint.
- `ginkgo -v ./test/e2e/suites/workspace -- --poll-interval=5s --poll-timeout=5m` runs the kubectl compatibility checks when the environment can actually serve the workspace surface exercised by this branch.

The automated suite covers the currently implemented workspace surface on this branch:

- the full completed phase-2 discovery baseline, without prematurely advertising deferred phase-3 CRD or cluster-scoped extensions
- workspace kubeconfig generation through `karmadactl workspace kubeconfig`
- `kubectl api-resources` over the supported workspace surface
- namespace creation and visibility through the workspace API
- CRUD-oriented flows for the writable desired-state resources, including RBAC, quotas, `serviceaccounts`, `networkpolicies`, `ingresses`, `poddisruptionbudgets`, and `horizontalpodautoscalers`
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

### Recommended Local-Up Verification Flow

For laptop verification without a pre-existing cluster:

1. Run `hack/local-up-karmada.sh`.
2. Export `KUBECONFIG=$HOME/.kube/karmada.config`.
3. Make sure `docker`, `kind`, and `cfssl` are available, and build `karmadactl` if `$(go env GOPATH)/bin/karmadactl` is not already present.
4. Deploy the workspace apiserver with `hack/deploy-workspace-apiserver.sh $HOME/.kube/karmada.config karmada-host $HOME/.kube/karmada.config karmada-apiserver`.
5. If you need a non-default image tag or registry, set `REGISTRY`, `VERSION`, `BUILD_FROM_SOURCE=false`, or `WAIT_FOR_READY=false` before the deploy step.
6. Run `hack/verify-workspace-local-up.sh <workspace>` if you want the scripted deploy plus test flow instead of stepping through the remaining commands manually.
7. Run `ginkgo -v ./test/e2e/suites/workspace -- --poll-interval=5s --poll-timeout=5m`.
8. Generate a workspace kubeconfig with `karmadactl workspace kubeconfig <workspace> > /tmp/<workspace>.kubeconfig` and use it with `kubectl` or `k9s`.
9. Clean up with `hack/undeploy-workspace-apiserver.sh $HOME/.kube/karmada.config karmada-host $HOME/.kube/karmada.config karmada-apiserver` when you are done.

The suite assumes all of the following are true:

- `KUBECONFIG` points at a Karmada control-plane kubeconfig
- `$(go env GOPATH)/bin/karmadactl` exists
- the target environment publishes the workspace proxy endpoint and truthful workspace discovery
- desired-state writes for supported namespaced resources, projected runtime reads, and live pod subresources are wired in the deployed workspace apiserver
- the control-plane kubeconfig can reach `placementviews.workspace.karmada.io` read endpoints when the phase-2 debug surface is deployed
- `WORKSPACE_E2E_AMBIGUOUS_POD` names a known ambiguous workspace-visible pod when running the ambiguity inspection and explicit-target-selection checks

If those prerequisites are not met, the suite skips instead of pretending the current workspace surface works.

## Offline Kubectl Smoke

The strongest no-cluster verification path on this branch is the offline discovery smoke in `pkg/workspace`.

- `go test ./pkg/workspace -run TestWorkspaceOfflineKubectlDiscoverySmoke -count=1`
- The test boots the workspace apiserver with `httptest`, writes a temporary kubeconfig, and shells out to `kubectl api-resources -o name`.
- It skips cleanly when no `kubectl` binary is available in the current shell.
- Set `WORKSPACE_KUBECTL_BIN=/path/to/kubectl` if you want to override which binary it uses.

Run it like this when you want a real client-level check without a cluster:

1. Confirm the kubectl binary the test will use: `command -v kubectl` or `WORKSPACE_KUBECTL_BIN=/path/to/kubectl command -v "$WORKSPACE_KUBECTL_BIN"`.
2. Run `go test ./pkg/workspace -run TestWorkspaceOfflineKubectlDiscoverySmoke -count=1 -v`.
3. Expect the test to pass only when the kubeconfig server points at the workspace proxy URL, not the apiserver root.
4. If you want to verify the helper wiring too, run `go test ./test/e2e/framework -run TestRunWorkspaceKubectlUsesConfiguredBinary -count=1 -v`.

## Handoff Checklist

When another person or machine has access to a real `local-up` environment, hand them this checklist:

1. Run `hack/local-up-karmada.sh`.
2. Run `hack/verify-workspace-local-up.sh <workspace>`.
3. Confirm whether the Ginkgo run exercised live specs or only skipped because `KUBECONFIG` or workspace readiness was missing.
4. Run `KUBECONFIG=$HOME/.kube/karmada.config karmadactl workspace kubeconfig <workspace> > /tmp/<workspace>.kubeconfig`.
5. Run `kubectl --kubeconfig /tmp/<workspace>.kubeconfig api-resources` and confirm unsupported surfaces such as `nodes`, `clusterroles`, and CRDs stay absent.
6. Run `k9s --kubeconfig /tmp/<workspace>.kubeconfig` and walk the manual smoke checklist below.
7. Run `hack/undeploy-workspace-apiserver.sh $HOME/.kube/karmada.config karmada-host $HOME/.kube/karmada.config karmada-apiserver` after the session.

## Manual `k9s` Smoke Checklist

`k9s` is a manual smoke bar only in phase 1. Do not treat the following as CI automation.

1. Generate a workspace kubeconfig with `karmadactl workspace kubeconfig <workspace> > /tmp/<workspace>.kubeconfig`.
2. Launch `k9s --kubeconfig /tmp/<workspace>.kubeconfig`.
3. Confirm the default views show logical workspace namespaces, workloads, pods, and events without exposing unsupported cluster-scoped surfaces such as nodes.
4. Navigate from a workload to a pod and verify log viewing works on the supported workload surface.
5. Verify shell and port-forward actions behave normally for a uniquely resolved pod target.
6. If an intentionally ambiguous live target is available, confirm the operation fails explicitly instead of silently selecting a backing cluster.
