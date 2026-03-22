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
source "${REPO_ROOT}"/hack/util.sh

CERT_DIR=${CERT_DIR:-"${HOME}/.karmada"}
REGISTRY=${REGISTRY:-"docker.io/karmada"}
VERSION=${VERSION:-"latest"}
BUILD_FROM_SOURCE=${BUILD_FROM_SOURCE:-"true"}
WORKSPACE_APISERVER_IMAGE="${REGISTRY}/karmada-workspace-apiserver:${VERSION}"

function usage() {
  echo "This script deploys karmada-workspace-apiserver on a local-up Karmada host cluster."
  echo "Usage: hack/deploy-workspace-apiserver.sh <HOST_CLUSTER_KUBECONFIG> <HOST_CONTEXT_NAME> <KARMADA_APISERVER_KUBECONFIG> <KARMADA_APISERVER_CONTEXT_NAME>"
  echo "Example: hack/deploy-workspace-apiserver.sh ~/.kube/karmada.config karmada-host ~/.kube/karmada.config karmada-apiserver"
  echo "Environment: BUILD_FROM_SOURCE=true|false (default true), REGISTRY=docker.io/karmada, VERSION=latest"
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
  echo -e "ERROR: failed to get kubernetes config file: '${HOST_CLUSTER_KUBECONFIG}', not existed.
"
  usage
  exit 1
fi
if ! kubectl config get-contexts "${HOST_CONTEXT_NAME}" --kubeconfig="${HOST_CLUSTER_KUBECONFIG}" > /dev/null 2>&1; then
  echo -e "ERROR: failed to get context: '${HOST_CONTEXT_NAME}' not in ${HOST_CLUSTER_KUBECONFIG}.
"
  usage
  exit 1
fi
if [[ ! -f "${KARMADA_APISERVER_KUBECONFIG}" ]]; then
  echo -e "ERROR: failed to get kubernetes config file: '${KARMADA_APISERVER_KUBECONFIG}', not existed.
"
  usage
  exit 1
fi
if ! kubectl config get-contexts "${KARMADA_APISERVER_CONTEXT_NAME}" --kubeconfig="${KARMADA_APISERVER_KUBECONFIG}" > /dev/null 2>&1; then
  echo -e "ERROR: failed to get context: '${KARMADA_APISERVER_CONTEXT_NAME}' not in ${KARMADA_APISERVER_KUBECONFIG}.
"
  usage
  exit 1
fi
if [[ ! -f "${CERT_DIR}/ca.crt" || ! -f "${CERT_DIR}/ca.key" || ! -f "${CERT_DIR}/ca-config.json" ]]; then
  echo "ERROR: expected Karmada CA files in ${CERT_DIR}. Run hack/local-up-karmada.sh first."
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

function create_workspace_cert_material() {
  util::cmd_must_exist_cfssl "v1.6.5"

  local workspace_apiserver_alt_names=(
    "karmada-workspace-apiserver.karmada-system.svc.cluster.local"
    "karmada-workspace-apiserver.karmada-system.svc"
    "localhost"
    "127.0.0.1"
  )

  util::create_certkey "" "${CERT_DIR}" "ca" karmada-workspace-apiserver "system:karmada:karmada-workspace-apiserver" "" "${workspace_apiserver_alt_names[@]}"
  util::create_certkey "" "${CERT_DIR}" "ca" karmada-workspace-apiserver-client "system:karmada:karmada-workspace-apiserver" "system:masters"
  util::create_certkey "" "${CERT_DIR}" "ca" karmada-workspace-apiserver-etcd-client "system:karmada:karmada-workspace-apiserver-etcd-client" "system:masters"
}

function apply_secret_template() {
  local src=$1
  local dest=$2
  cp "${src}" "${dest}"
  shift 2
  while [[ $# -gt 0 ]]; do
    local key=$1
    local value=$2
    sed -i'' -e "s|${key}|${value}|g" "${dest}"
    shift 2
  done
  kubectl --context="${HOST_CONTEXT_NAME}" apply -f "${dest}"
}

function deploy_workspace_secrets() {
  local temp_dir=$1
  local karmada_ca karmada_workspace_server_crt karmada_workspace_server_key
  local karmada_workspace_client_crt karmada_workspace_client_key
  local karmada_workspace_etcd_client_crt karmada_workspace_etcd_client_key

  karmada_ca=$(base64 < "${CERT_DIR}/ca.crt" | tr -d '\r\n')
  karmada_workspace_server_crt=$(base64 < "${CERT_DIR}/karmada-workspace-apiserver.crt" | tr -d '\r\n')
  karmada_workspace_server_key=$(base64 < "${CERT_DIR}/karmada-workspace-apiserver.key" | tr -d '\r\n')
  karmada_workspace_client_crt=$(base64 < "${CERT_DIR}/karmada-workspace-apiserver-client.crt" | tr -d '\r\n')
  karmada_workspace_client_key=$(base64 < "${CERT_DIR}/karmada-workspace-apiserver-client.key" | tr -d '\r\n')
  karmada_workspace_etcd_client_crt=$(base64 < "${CERT_DIR}/karmada-workspace-apiserver-etcd-client.crt" | tr -d '\r\n')
  karmada_workspace_etcd_client_key=$(base64 < "${CERT_DIR}/karmada-workspace-apiserver-etcd-client.key" | tr -d '\r\n')

  apply_secret_template "${REPO_ROOT}/artifacts/deploy/karmada-config-secret.yaml" "${temp_dir}/karmada-workspace-apiserver-config-secret.yaml"     '\${component}' 'karmada-workspace-apiserver'     '\${ca_crt}' "${karmada_ca}"     '\${client_crt}' "${karmada_workspace_client_crt}"     '\${client_key}' "${karmada_workspace_client_key}"

  apply_secret_template "${REPO_ROOT}/artifacts/deploy/karmada-cert-secret.yaml" "${temp_dir}/karmada-workspace-apiserver-cert-secret.yaml"     '\${name}' 'karmada-workspace-apiserver'     '\${ca_crt}' "${karmada_ca}"     '\${tls_crt}' "${karmada_workspace_server_crt}"     '\${tls_key}' "${karmada_workspace_server_key}"

  apply_secret_template "${REPO_ROOT}/artifacts/deploy/karmada-cert-secret.yaml" "${temp_dir}/karmada-workspace-apiserver-etcd-client-cert-secret.yaml"     '\${name}' 'karmada-workspace-apiserver-etcd-client'     '\${ca_crt}' "${karmada_ca}"     '\${tls_crt}' "${karmada_workspace_etcd_client_crt}"     '\${tls_key}' "${karmada_workspace_etcd_client_key}"
}

function build_and_load_workspace_apiserver_image() {
  if [[ "${BUILD_FROM_SOURCE}" == "true" ]]; then
    make karmada-workspace-apiserver GOOS=linux --directory="${REPO_ROOT}"
    VERSION="${VERSION}" REGISTRY="${REGISTRY}" BUILD_PLATFORMS=linux/$(go env GOARCH) "${REPO_ROOT}/hack/docker.sh" karmada-workspace-apiserver
  fi
  util::cmd_must_exist kind
  kind load docker-image "${WORKSPACE_APISERVER_IMAGE}" --name="${HOST_CONTEXT_NAME}"
}

function render_workspace_apiserver_manifest() {
  local temp_dir=$1
  local rendered_manifest="${temp_dir}/karmada-workspace-apiserver.yaml"

  cp "${REPO_ROOT}/artifacts/deploy/karmada-workspace-apiserver.yaml" "${rendered_manifest}"
  sed -i'' -e "s|docker.io/karmada/karmada-workspace-apiserver:latest|${WORKSPACE_APISERVER_IMAGE}|g" "${rendered_manifest}"

  echo "${rendered_manifest}"
}

export KUBECONFIG="${HOST_CLUSTER_KUBECONFIG}"
kubectl --context="${HOST_CONTEXT_NAME}" apply -f "${REPO_ROOT}/artifacts/deploy/namespace.yaml"

build_and_load_workspace_apiserver_image
create_workspace_cert_material

TEMP_PATH=$(mktemp -d)
trap 'rm -rf "${TEMP_PATH}"; recover_kubeconfig' EXIT

deploy_workspace_secrets "${TEMP_PATH}"
rendered_workspace_manifest=$(render_workspace_apiserver_manifest "${TEMP_PATH}")
kubectl --context="${HOST_CONTEXT_NAME}" apply -f "${rendered_workspace_manifest}"
util::wait_pod_ready "${HOST_CONTEXT_NAME}" "${KARMADA_WORKSPACE_APISERVER_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"

workspace_ca=$(base64 < "${CERT_DIR}/ca.crt" | tr -d '\r\n')
cp "${REPO_ROOT}/artifacts/deploy/karmada-workspace-apiserver-apiservice.yaml" "${TEMP_PATH}/karmada-workspace-apiserver-apiservice.yaml"
sed -i'' -e "s/{{caBundle}}/${workspace_ca}/g" "${TEMP_PATH}/karmada-workspace-apiserver-apiservice.yaml"

export KUBECONFIG="${KARMADA_APISERVER_KUBECONFIG}"
kubectl --context="${KARMADA_APISERVER_CONTEXT_NAME}" apply -f "${TEMP_PATH}/karmada-workspace-apiserver-apiservice.yaml"
util::wait_apiservice_ready "${KARMADA_APISERVER_CONTEXT_NAME}" "${KARMADA_WORKSPACE_APISERVER_LABEL}"

echo "Karmada workspace apiserver is deployed successfully."
