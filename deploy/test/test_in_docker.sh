#!/bin/bash
set -xeuo pipefail

# Export KIND early so check_env_var in sourced scripts can see it
export KIND="${KIND:-false}"

. utils.sh
. kind_helpers.sh

# Force fresh kind cluster in CI by default; allow override by environment
export KIND_FORCE_CLEAN="${KIND_FORCE_CLEAN:-true}"
export CONJUR_START_TIMEOUT="${CONJUR_START_TIMEOUT:-5m}"

# Use a per-run docker network so parallel KinD runs do not tear down each other's clusters during cleanup.
export KIND_DOCKER_NETWORK="${KIND_DOCKER_NETWORK:-kind-network-${UNIQUE_TEST_ID:-secrets-provider-test}}"

cleanup_kind_network() {
  local cluster_name
  cluster_name="$(kind_cluster_name)"
  local control_plane_container="${cluster_name}-control-plane"
  
  # First, clean up any orphaned control-plane containers from this test run
  if docker inspect "${control_plane_container}" > /dev/null 2>&1; then
    echo "Removing control-plane container: ${control_plane_container}"
    docker rm -f -v "${control_plane_container}" || true
  fi
  
  # Then clean up the docker network and any attached containers
  if docker network inspect "${KIND_DOCKER_NETWORK}" > /dev/null 2>&1; then
    containers="$(docker network inspect --format '{{range .Containers}}{{.Name}} {{end}}' "${KIND_DOCKER_NETWORK}")"
    for container in ${containers}; do
      echo "Removing container from network: ${container}"
      docker rm -f -v "${container}" || true
    done
    echo "Removing docker network: ${KIND_DOCKER_NETWORK}"
    docker network rm "${KIND_DOCKER_NETWORK}" || true
  fi
}

kind_cluster_name() {
  kind_sanitize_cluster_name "${UNIQUE_TEST_ID:-secrets-provider-test}"
}

kind_cluster_exists() {
  local name
  name="$(kind_cluster_name)"
  docker inspect "${name}-control-plane" > /dev/null 2>&1
}

capture_kind_diagnostics() {
  if [[ "${KIND}" != "true" ]]; then
    return 0
  fi
  if ! kind_cluster_exists; then
    echo "KinD cluster $(kind_cluster_name) not found; skipping diagnostics"
    return 0
  fi

  local ns="${CONJUR_NAMESPACE_NAME:-}"
  local app_ns="${APP_NAMESPACE_NAME:-}"
  local id="${UNIQUE_TEST_ID:-unknown}"
  local out_dir="output"
  mkdir -p "${out_dir}" || true

  echo "Capturing KinD diagnostics into ${out_dir}/ (id=${id}, ns=${ns})"

  # Run diagnostics from inside the existing cluster (SKIP_KIND_CREATE to avoid creating new cluster)
  SKIP_KIND_CREATE=true runDockerCommand "
    set +e
    ts=\$(date -u +%Y%m%dT%H%M%SZ)
    prefix='/src/deploy/${out_dir}/kind-${id}-'\${ts}

    echo '=== kind diagnostics ${id} '\${ts}' ===' > \${prefix}.summary.txt
    echo 'TEST_PLATFORM=${TEST_PLATFORM:-}' >> \${prefix}.summary.txt
    echo 'KIND_DOCKER_NETWORK=${KIND_DOCKER_NETWORK:-}' >> \${prefix}.summary.txt
    echo 'CONJUR_NAMESPACE_NAME=${ns}' >> \${prefix}.summary.txt
    echo 'APP_NAMESPACE_NAME=${app_ns}' >> \${prefix}.summary.txt

    kubectl version --client=true >> \${prefix}.summary.txt 2>&1 || true
    kubectl cluster-info > \${prefix}.cluster-info.txt 2>&1 || true
    kubectl get nodes -o wide > \${prefix}.nodes.txt 2>&1 || true
    kubectl get namespaces > \${prefix}.namespaces.txt 2>&1 || true
    kubectl get pods -A -o wide > \${prefix}.pods-all.txt 2>&1 || true
    kubectl get events -A --sort-by='.lastTimestamp' > \${prefix}.events-all.txt 2>&1 || true

    if [[ -n "${ns}" ]]; then
      kubectl -n "${ns}" get pods -o wide > \${prefix}.pods.txt 2>&1 || true
      kubectl -n "${ns}" get deploy,rs,sts,svc -o wide > \${prefix}.workloads.txt 2>&1 || true
      kubectl -n "${ns}" get events --sort-by='.lastTimestamp' > \${prefix}.events.txt 2>&1 || true
      kubectl -n "${ns}" describe deploy/conjur-cluster > \${prefix}.deploy-describe.txt 2>&1 || true
      kubectl -n "${ns}" describe pods > \${prefix}.pods-describe.txt 2>&1 || true
      kubectl -n "${ns}" logs -l app=conjur-cluster --all-containers --tail=2000 > \${prefix}.conjur-cluster.logs.txt 2>&1 || true
      kubectl -n "${ns}" logs -l app=conjur-cli --all-containers --tail=2000 > \${prefix}.conjur-cli.logs.txt 2>&1 || true
    fi

    if [[ -n "${app_ns}" ]] && kubectl get namespace "${app_ns}" >/dev/null 2>&1; then
      kubectl -n "${app_ns}" get pods -o wide > \${prefix}.app-pods.txt 2>&1 || true
      kubectl -n "${app_ns}" get events --sort-by='.lastTimestamp' > \${prefix}.app-events.txt 2>&1 || true
      kubectl -n "${app_ns}" describe pods > \${prefix}.app-pods-describe.txt 2>&1 || true
      kubectl -n "${app_ns}" logs -l app=secrets-provider --all-containers --tail=2000 > \${prefix}.secrets-provider.logs.txt 2>&1 || true
      kubectl -n "${app_ns}" logs -l app=test-env --all-containers --tail=2000 > \${prefix}.test-env.logs.txt 2>&1 || true
    fi
  " || true
}

# Clean up when script completes and fails
finish() {
  exit_code=$?
  
  announce 'Wrapping up and removing test environment'

  if [[ ${exit_code} -ne 0 ]]; then
    echo "Non-zero exit detected (${exit_code}); capturing diagnostics before cleanup"
    capture_kind_diagnostics
  fi

  # Stop the running processes
  if [[ "${KIND}" = "true" ]]; then
    # Do not force-clean during stop/teardown; we want to tear down resources in the
    # cluster that was used for this test run, not delete/recreate a new cluster.
    if ! kind_cluster_exists; then
      echo "KinD cluster $(kind_cluster_name) not found; skipping ./stop cleanup"
    else
      SKIP_KIND_CREATE=true KIND_FORCE_CLEAN=false runDockerCommand "
        ./stop && cd kubernetes-conjur-deploy-$UNIQUE_TEST_ID && DEV=true ./stop
      "
    fi

    # Ensure we don't leak kind containers / network
    cleanup_kind_network
  else
    runDockerCommand "
      ./stop && cd kubernetes-conjur-deploy-$UNIQUE_TEST_ID && ./stop
    "
  fi
  # Remove the deploy directory
  rm -rf "kubernetes-conjur-deploy-$UNIQUE_TEST_ID"
}
trap finish EXIT

main() {
  mkdir -p output #location where Secrets Provider/Conjur logs will be saved
  if [[ "${KIND}" = "true" ]]; then
    docker network create "${KIND_DOCKER_NETWORK}" || true
  fi

  buildTestRunnerImage

  deployConjur
  
  # After Conjur is deployed, prevent subsequent commands from recreating the cluster
  if [[ "${KIND}" = "true" ]]; then
    export KIND_FORCE_CLEAN=false
  fi
  
  deployTest
}

buildTestRunnerImage() {
  pushd ..
  docker build --tag ${TEST_RUNNER_IMAGE}:${CONJUR_NAMESPACE_NAME} \
    --file Dockerfile.e2e \
    --build-arg OPENSHIFT_CLI_URL="${OPENSHIFT_CLI_URL:-}" \
    --build-arg KUBECTL_CLI_URL="${KUBECTL_CLI_URL:-}" \
    .
  popd
}

run_conjur_deploy_in_kind() {
  local cmd="$1"
  local cluster_name="$(kind_sanitize_cluster_name "${UNIQUE_TEST_ID:-secrets-provider-test}")"
  local kind_version="v0.31.0"
  
  # Export KIND_CLUSTER_NAME so it gets passed to the container via runDockerCommand
  export KIND_CLUSTER_NAME="${cluster_name}"
  
  runDockerCommand "set -e
    # Ensure kind binary is available in runner
    if ! command -v kind >/dev/null 2>&1; then
      echo \"Downloading kind binary...\"
      curl -Lo /usr/local/bin/kind https://kind.sigs.k8s.io/dl/${kind_version}/kind-linux-amd64
      chmod +x /usr/local/bin/kind
    fi

    cd ./kubernetes-conjur-deploy-$UNIQUE_TEST_ID
    
    export KIND=true
    export KIND_CLUSTER_NAME=${cluster_name}
    export LOG_LEVEL=debug
    export TEST_PLATFORM=kubernetes
    export CONJUR_NAMESPACE_NAME=${CONJUR_NAMESPACE_NAME}
    
    $cmd"
}

# Main deployment logic
deployConjur() {
  # Prepare Docker images
  # This is done outside of the container to avoid authentication errors when pulling from the internal registry
  # from inside the container
  if [[ "${CONJUR_DEPLOYMENT}" != "oss" && -n "${CONJUR_APPLIANCE_IMAGE:-}" ]]; then
    docker pull $CONJUR_APPLIANCE_IMAGE
  fi

  git clone --single-branch --branch master \
      https://github.com/cyberark/kubernetes-conjur-deploy.git \
      kubernetes-conjur-deploy-$UNIQUE_TEST_ID

  # Build deployment command
  local deploy_cmd="./start"
  if [ "$CONJUR_DEPLOYMENT" = "oss" ]; then
    deploy_cmd="$deploy_cmd --oss"
  fi
  
  # Run deployment based on platform
  if [[ "${KIND}" = "true" ]]; then
    run_conjur_deploy_in_kind "$deploy_cmd"
  else
    runDockerCommand "cd ./kubernetes-conjur-deploy-$UNIQUE_TEST_ID && $deploy_cmd"
  fi
}
deployTest() {
  runDockerCommand "./run_with_summon.sh"
}

main
