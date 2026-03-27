#!/bin/bash
set -euo pipefail

# Helper functions to create KinD clusters, wait for readiness, and load images
# Usage: source this file and call kind_create_cluster <name>

kind_sanitize_cluster_name() {
  local raw_name=${1:-secrets-provider-test}
  local name
  name="$(printf "%s" "${raw_name}" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9-]/-/g' | sed -E 's/^-+|-+$//g')"
  if [[ -z "${name}" ]]; then
    name="secrets-provider-test"
  fi
  printf "%s" "${name}"
}

kind_create_cluster() {
  local name=${1:-secrets-provider-test}
  echo "Creating kind cluster: ${name}"

  if ! command -v kind >/dev/null 2>&1; then
    echo "kind binary not found in PATH"
    return 1
  fi

  # Ensure the KinD nodes attach to the same docker network as the runner container.
  export KIND_EXPERIMENTAL_DOCKER_NETWORK="${KIND_DOCKER_NETWORK:-kind-network}"
  echo "Using KIND_EXPERIMENTAL_DOCKER_NETWORK=${KIND_EXPERIMENTAL_DOCKER_NETWORK}"

  local container_name="${name}-control-plane"
  
  # Check if cluster exists in KIND's registry
  if kind get clusters | grep -q "^${name}$"; then
    if [[ "${KIND_FORCE_CLEAN:-false}" = "true" ]]; then
      echo "Kind cluster ${name} exists; deleting because KIND_FORCE_CLEAN=true"
      kind delete cluster --name "${name}" || true
    else
      echo "Kind cluster ${name} already exists; skipping (set KIND_FORCE_CLEAN=true to force recreation)"
      return 0
    fi
  fi

  # Clean up any orphaned control-plane container to prevent "node(s) already exist" error
  if docker inspect "${container_name}" > /dev/null 2>&1; then
    echo "Removing orphaned container ${container_name}"
    docker rm -f -v "${container_name}" || true
  fi

  kind create cluster --name "${name}"
}

kind_wait_ready() {
  local kubeconfig=${1:?"kubeconfig path required"}
  local timeout_sec=${2:-120}
  local interval_sec=${KIND_WAIT_INTERVAL:-2}
  echo "Waiting up to ${timeout_sec}s for cluster readiness..."

  local elapsed=0
  until KUBECONFIG="${kubeconfig}" kubectl cluster-info > /dev/null 2>&1; do
    sleep "${interval_sec}"
    elapsed=$((elapsed+interval_sec))
    if [[ ${elapsed} -ge ${timeout_sec} ]]; then
      echo "Timed out waiting for kubectl cluster-info"
      echo "------ Diagnostic: kubectl cluster-info (raw) ------"
      KUBECONFIG="${kubeconfig}" kubectl cluster-info || true
      echo "------ Diagnostic: kubectl get nodes -o wide ------"
      KUBECONFIG="${kubeconfig}" kubectl get nodes -o wide || true
      return 1
    fi
  done

  # Wait for nodes to be Ready
  KUBECONFIG="${kubeconfig}" kubectl wait --for=condition=Ready nodes --all --timeout="${timeout_sec}s"
}

kind_load_image() {
  local name="${1:-secrets-provider-test}"
  local image="${2:?image is required}"
  local control_plane_container="${name}-control-plane"
  local attempt

  echo "Loading image ${image} into kind cluster ${name}"

  if kind load docker-image "${image}" --name "${name}"; then
    return 0
  fi
  echo "kind load failed for ${image}; falling back to containerd import..."

  if ! docker inspect "${control_plane_container}" >/dev/null 2>&1; then
    echo "ERROR: Could not find KinD control-plane container ${control_plane_container}"
    return 1
  fi

  if ! docker exec "${control_plane_container}" test -S /run/containerd/containerd.sock; then
    echo "Timed out waiting for ${control_plane_container} containerd socket."
    return 1
  fi

  for attempt in 1 2 3; do
    if docker save "${image}" | \
      docker exec --privileged -i "${control_plane_container}" \
        ctr --namespace=k8s.io images import --digests --snapshotter=overlayfs -; then
      return 0
    fi
    echo "containerd import failed for ${image} (attempt ${attempt}/3); retrying..."
    sleep 2
  done

  return 1
}
