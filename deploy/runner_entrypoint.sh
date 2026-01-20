#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

source ./kind_helpers.sh || true

if [[ "${KIND}" = "true" ]]; then
  export KIND_DOCKER_NETWORK="${KIND_DOCKER_NETWORK:-kind-network}"

  # Sanitize the cluster name to safe characters and provide a default
  raw_name="${UNIQUE_TEST_ID:-secrets-provider-test}"
  KIND_CLUSTER_NAME="$(kind_sanitize_cluster_name "${raw_name}")"
  export KIND_CLUSTER_NAME
  
  # Allow skipping cluster creation for diagnostics collection in existing cluster
  if [[ "${SKIP_KIND_CREATE:-false}" != "true" ]]; then
    kind_create_cluster "${KIND_CLUSTER_NAME}"
  else
    echo "SKIP_KIND_CREATE=true; connecting to existing cluster ${KIND_CLUSTER_NAME}"
  fi

  # Write kubeconfig to a secure temporary file and export it
  KIND_KUBECONFIG_PATH="$(mktemp -p /tmp kubeconfig-kind-${KIND_CLUSTER_NAME}.XXXXXX)"
  chmod 600 "${KIND_KUBECONFIG_PATH}" || true
  kind get kubeconfig --name "${KIND_CLUSTER_NAME}" > "${KIND_KUBECONFIG_PATH}"
  export KUBECONFIG="${KIND_KUBECONFIG_PATH}"

  control_plane_container="$(kind get nodes --name "${KIND_CLUSTER_NAME}" 2>/dev/null | head -n 1 || true)"
  if [[ -z "${control_plane_container}" ]]; then
    control_plane_container="${KIND_CLUSTER_NAME}-control-plane"
  fi
  cp_ip="$(docker inspect -f "{{(index .NetworkSettings.Networks \"${KIND_DOCKER_NETWORK}\").IPAddress}}" "${control_plane_container}" 2>/dev/null || true)"
  if [[ -n "${cp_ip}" ]]; then
    cluster_name="$(kubectl --kubeconfig="${KUBECONFIG}" config view -o jsonpath='{.clusters[0].name}' 2>/dev/null || true)"
    if [[ -n "${cluster_name}" ]]; then
      kubectl --kubeconfig="${KUBECONFIG}" config set-cluster "${cluster_name}" --server="https://${cp_ip}:6443" >/dev/null
      new_server="$(kubectl --kubeconfig="${KUBECONFIG}" config view -o jsonpath='{.clusters[0].cluster.server}' 2>/dev/null || true)"
      if [[ "${new_server}" != "https://${cp_ip}:6443" ]]; then
        echo "Error: failed to rewrite kubeconfig server. Expected https://${cp_ip}:6443, got ${new_server:-<empty>}"
        echo "KUBECONFIG=$(basename "${KUBECONFIG}")"
        exit 1
      fi
      echo "Rewrote kubeconfig cluster server to https://${cp_ip}:6443 for container ${control_plane_container}"
    fi
  else
    echo "Warning: could not determine control-plane IP for ${control_plane_container}; continuing with original kubeconfig"
  fi

  # Now wait for the cluster to be ready using the adjusted kubeconfig
  kind_wait_ready "${KUBECONFIG}" 180
  
  # Load Conjur appliance image if needed (DAP only, not OSS)
  if [[ "${CONJUR_DEPLOYMENT:-dap}" != "oss" && -n "${CONJUR_APPLIANCE_IMAGE:-}" ]]; then
    echo "Loading ${CONJUR_APPLIANCE_IMAGE} into Kind cluster ${KIND_CLUSTER_NAME}..."
    # Verify image exists
    if ! docker image inspect "${CONJUR_APPLIANCE_IMAGE}" > /dev/null 2>&1; then
      echo "Image ${CONJUR_APPLIANCE_IMAGE} not found in Docker daemon, attempting to pull..."
      docker pull "${CONJUR_APPLIANCE_IMAGE}"
    fi
    
    # Load directly without --all-platforms to avoid multi-arch manifest issues
    # The minus sign (-) tells ctr to read from stdin
    echo "Loading image into node ${KIND_CLUSTER_NAME}-control-plane..."
    docker save "${CONJUR_APPLIANCE_IMAGE}" | \
      docker exec -i "${KIND_CLUSTER_NAME}-control-plane" \
        ctr --namespace=k8s.io images import --digests --snapshotter=overlayfs -
    
    # Also tag the image with the expected name that kubernetes-conjur-deploy will use
    # Format: conjur-appliance:<namespace>
    expected_tag="conjur-appliance:${CONJUR_NAMESPACE_NAME}"
    echo "Tagging image as ${expected_tag} for kubernetes-conjur-deploy..."
    docker tag "${CONJUR_APPLIANCE_IMAGE}" "${expected_tag}"
    docker save "${expected_tag}" | \
      docker exec -i "${KIND_CLUSTER_NAME}-control-plane" \
        ctr --namespace=k8s.io images import --digests --snapshotter=overlayfs -
    
    echo "Successfully loaded ${CONJUR_APPLIANCE_IMAGE}"
  fi
  
  # Validate kubeconfig connectivity after rewrite
  if ! kubectl --kubeconfig="${KUBECONFIG}" cluster-info > /dev/null 2>&1; then
    echo "ERROR: kubectl cannot connect to cluster after kubeconfig rewrite"
    echo "Kubeconfig server: $(kubectl --kubeconfig="${KUBECONFIG}" config view -o jsonpath='{.clusters[0].cluster.server}')"
    kubectl --kubeconfig="${KUBECONFIG}" cluster-info || true
    exit 1
  fi
  echo "Validated kubeconfig connectivity to KinD cluster"
else
  ./platform_login.sh
fi

exec "$@"
