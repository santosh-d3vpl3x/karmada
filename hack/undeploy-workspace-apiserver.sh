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
CERT_DIR=${CERT_DIR:-"${HOME}/.karmada"}
WORKSPACE_CERT_FILES=(
  "karmada-workspace-apiserver.crt"
  "karmada-workspace-apiserver.key"
  "karmada-workspace-apiserver-client.crt"
  "karmada-workspace-apiserver-client.key"
  "karmada-workspace-apiserver-etcd-client.crt"
  "karmada-workspace-apiserver-etcd-client.key"
)

function usage() {
  echo "This script removes karmada-workspace-apiserver from a local-up Karmada environment."
  echo "Usage: hack/undeploy-workspace-apiserver.sh <HOST_CLUSTER_KUBECONFIG> <HOST_CONTEXT_NAME> <KARMADA_APISERVER_KUBECONFIG> <KARMADA_APISERVER_CONTEXT_NAME>"
  echo "Example: hack/undeploy-workspace-apiserver.sh ~/.kube/karmada.config karmada-host ~/.kube/karmada.config karmada-apiserver"
}

if [[ $# -ne 4 ]]; then
  usage
  exit 1
fi

HOST_CLUSTER_KUBECONFIG=$1
HOST_CONTEXT_NAME=$2
KARMADA_APISERVER_KUBECONFIG=$3
KARMADA_APISERVER_CONTEXT_NAME=$4

if [[ ! -f "${HOST_CLUSTER_KUBECONFIG}" ]]; then
  echo -e "ERROR: failed to get kubernetes config file: '${HOST_CLUSTER_KUBECONFIG}', not existed.\n"
  usage
  exit 1
fi
if ! kubectl config get-contexts "${HOST_CONTEXT_NAME}" --kubeconfig="${HOST_CLUSTER_KUBECONFIG}" > /dev/null 2>&1; then
  echo -e "ERROR: failed to get context: '${HOST_CONTEXT_NAME}' not in ${HOST_CLUSTER_KUBECONFIG}.\n"
  usage
  exit 1
fi
if [[ ! -f "${KARMADA_APISERVER_KUBECONFIG}" ]]; then
  echo -e "ERROR: failed to get kubernetes config file: '${KARMADA_APISERVER_KUBECONFIG}', not existed.\n"
  usage
  exit 1
fi
if ! kubectl config get-contexts "${KARMADA_APISERVER_CONTEXT_NAME}" --kubeconfig="${KARMADA_APISERVER_KUBECONFIG}" > /dev/null 2>&1; then
  echo -e "ERROR: failed to get context: '${KARMADA_APISERVER_CONTEXT_NAME}' not in ${KARMADA_APISERVER_KUBECONFIG}.\n"
  usage
  exit 1
fi

if [ -n "${KUBECONFIG+x}" ]; then
  CURR_KUBECONFIG=$KUBECONFIG
fi

function recover_kubeconfig() {
  if [ -n "${CURR_KUBECONFIG+x}" ]; then
    export KUBECONFIG="${CURR_KUBECONFIG}"
  else
    unset KUBECONFIG
  fi
}
trap recover_kubeconfig EXIT

export KUBECONFIG="${KARMADA_APISERVER_KUBECONFIG}"
kubectl --context="${KARMADA_APISERVER_CONTEXT_NAME}" delete -f "${REPO_ROOT}/artifacts/deploy/karmada-workspace-apiserver-apiservice.yaml" --ignore-not-found

export KUBECONFIG="${HOST_CLUSTER_KUBECONFIG}"
kubectl --context="${HOST_CONTEXT_NAME}" delete -f "${REPO_ROOT}/artifacts/deploy/karmada-workspace-apiserver.yaml" --ignore-not-found
for secret_name in \
  karmada-workspace-apiserver-config \
  karmada-workspace-apiserver-cert \
  karmada-workspace-apiserver-etcd-client-cert; do
  kubectl --context="${HOST_CONTEXT_NAME}" delete secret "${secret_name}" -n karmada-system --ignore-not-found
 done

for cert_file in "${WORKSPACE_CERT_FILES[@]}"; do
  rm -f "${CERT_DIR}/${cert_file}"
done

echo "Karmada workspace apiserver is undeployed successfully."
