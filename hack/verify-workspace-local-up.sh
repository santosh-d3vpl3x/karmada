#!/usr/bin/env bash
# Copyright 2026 The Karmada Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
WORKSPACE_NAME=${1:-}
KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
HOST_CLUSTER_KUBECONFIG=${HOST_CLUSTER_KUBECONFIG:-"${KUBECONFIG_PATH}/karmada.config"}
HOST_CONTEXT_NAME=${HOST_CONTEXT_NAME:-"karmada-host"}
KARMADA_APISERVER_KUBECONFIG=${KARMADA_APISERVER_KUBECONFIG:-"${HOST_CLUSTER_KUBECONFIG}"}
KARMADA_APISERVER_CONTEXT_NAME=${KARMADA_APISERVER_CONTEXT_NAME:-"karmada-apiserver"}
WORKSPACE_DEPLOY_SCRIPT=${WORKSPACE_DEPLOY_SCRIPT:-"${REPO_ROOT}/hack/deploy-workspace-apiserver.sh"}
RUN_GINKGO=${RUN_GINKGO:-"true"}

function usage() {
  echo "This script deploys the workspace apiserver onto a local-up Karmada environment and runs the workspace verification flow."
  echo "Usage: hack/verify-workspace-local-up.sh <WORKSPACE_NAME>"
  echo "Environment overrides: HOST_CLUSTER_KUBECONFIG, HOST_CONTEXT_NAME, KARMADA_APISERVER_KUBECONFIG, KARMADA_APISERVER_CONTEXT_NAME, WORKSPACE_DEPLOY_SCRIPT, RUN_GINKGO"
}

if [[ -z "${WORKSPACE_NAME}" ]]; then
  usage
  exit 1
fi

"${WORKSPACE_DEPLOY_SCRIPT}" "${HOST_CLUSTER_KUBECONFIG}" "${HOST_CONTEXT_NAME}" "${KARMADA_APISERVER_KUBECONFIG}" "${KARMADA_APISERVER_CONTEXT_NAME}"

go test ./test/e2e/suites/workspace -count=1

if [[ "${RUN_GINKGO}" == "true" ]]; then
  GOPATH_BIN=$(go env GOPATH)/bin/ginkgo
  if [[ -x "${GOPATH_BIN}" ]]; then
    "${GOPATH_BIN}" -v ./test/e2e/suites/workspace -- --poll-interval=5s --poll-timeout=5m
  elif command -v ginkgo > /dev/null 2>&1; then
    ginkgo -v ./test/e2e/suites/workspace -- --poll-interval=5s --poll-timeout=5m
  else
    echo "Skipping Ginkgo because no ginkgo binary is available."
  fi
else
  echo "Skipping Ginkgo because RUN_GINKGO=${RUN_GINKGO}."
fi

echo

echo "Manual handoff commands:"
echo "  KUBECONFIG=${HOST_CLUSTER_KUBECONFIG} karmadactl workspace kubeconfig ${WORKSPACE_NAME} > /tmp/${WORKSPACE_NAME}.kubeconfig"
echo "  kubectl --kubeconfig /tmp/${WORKSPACE_NAME}.kubeconfig api-resources"
echo "  kubectl --kubeconfig /tmp/${WORKSPACE_NAME}.kubeconfig get namespaces"
echo "  k9s --kubeconfig /tmp/${WORKSPACE_NAME}.kubeconfig"
