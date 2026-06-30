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
  # shellcheck disable=SC2027,SC2086
  # (SC2027/SC2086: intentional mixed quoting in nested shell - outer shell expands ${ns}/${app_ns},
  #  inner shell expands \${prefix}; namespace values are guaranteed to have no spaces)
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
    kubectl get events -A --sort-by='.lastTimestamp' \
      | grep -E 'Warning|BackOff|CrashLoopBackOff|Failed|Evicted|Error|deadline|timeout' \
      > \${prefix}.events-all-warnings.txt 2>&1 || true

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
      kubectl -n "${app_ns}" get pod -o custom-columns=NAME:.metadata.name,PHASE:.status.phase,RESTARTS:.status.containerStatuses[*].restartCount,READY:.status.containerStatuses[*].ready,STATE:.status.containerStatuses[*].state.waiting.reason,NODE:.spec.nodeName > \${prefix}.app-pod-statuses.txt 2>&1 || true
      kubectl -n "${app_ns}" get deploy,rs,job -o wide > \${prefix}.app-workloads.txt 2>&1 || true
      kubectl -n "${app_ns}" get events --sort-by='.lastTimestamp' > \${prefix}.app-events.txt 2>&1 || true
      kubectl -n "${app_ns}" get events --sort-by='.lastTimestamp' \
        | grep -E 'Warning|BackOff|CrashLoopBackOff|Failed|Evicted|Error|deadline|timeout' \
        > \${prefix}.app-events-warnings.txt 2>&1 || true
      kubectl -n "${app_ns}" describe pods > \${prefix}.app-pods-describe.txt 2>&1 || true
      kubectl -n "${app_ns}" describe deployment secrets-provider-reloader > \${prefix}.reloader-deploy-describe.txt 2>&1 || true
      kubectl -n "${app_ns}" describe rs -l app.kubernetes.io/name=reloader > \${prefix}.reloader-rs-describe.txt 2>&1 || true
      kubectl -n "${app_ns}" describe pod -l app.kubernetes.io/name=reloader > \${prefix}.reloader-pods-describe.txt 2>&1 || true
      kubectl -n "${app_ns}" get secrets -o name > \${prefix}.app-secrets.txt 2>&1 || true
      kubectl -n "${app_ns}" get secret test-k8s-secret -o yaml > \${prefix}.test-k8s-secret.yaml 2>&1 || true
      kubectl -n "${app_ns}" get secret test-k8s-secret-fetch-all -o yaml > \${prefix}.test-k8s-secret-fetch-all.yaml 2>&1 || true
      kubectl -n "${app_ns}" get secret test-k8s-secret-fetch-all-base64 -o yaml > \${prefix}.test-k8s-secret-fetch-all-base64.yaml 2>&1 || true
      kubectl -n "${app_ns}" get secret test-k8s-secret-with-templates -o yaml > \${prefix}.test-k8s-secret-with-templates.yaml 2>&1 || true
      kubectl -n "${app_ns}" get secret labeled-k8s-secret -o yaml > \${prefix}.labeled-k8s-secret.yaml 2>&1 || true
      kubectl -n "${app_ns}" logs -l app=secrets-provider --all-containers --tail=2000 > \${prefix}.secrets-provider.logs.txt 2>&1 || true
      kubectl -n "${app_ns}" logs -l app=secrets-provider --all-containers --previous --tail=2000 > \${prefix}.secrets-provider.previous.logs.txt 2>&1 || true
      kubectl -n "${app_ns}" logs -l app=test-env --all-containers --tail=2000 > \${prefix}.test-env.logs.txt 2>&1 || true
      kubectl -n "${app_ns}" logs -l app.kubernetes.io/name=reloader --all-containers --tail=2000 > \${prefix}.reloader.logs.txt 2>&1 || true
      kubectl -n "${app_ns}" logs -l app.kubernetes.io/name=reloader --all-containers --previous --tail=2000 > \${prefix}.reloader.previous.logs.txt 2>&1 || true
    fi
  " || true
}

# Clean up when script completes and fails
finish() {
  exit_code=$?

  if [[ "${DEV}" = "true" ]]; then
    return 0
  fi
  
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
        ./stop
      "
      if [[ "${CONJUR_DEPLOYMENT}" != "cloud" ]]; then
        SKIP_KIND_CREATE=true KIND_FORCE_CLEAN=false runDockerCommand "
          cd kubernetes-conjur-deploy-$UNIQUE_TEST_ID && DEV=true ./stop
        "
      fi
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
  if [[ "${DEV}" = "true" && "${RELOAD_ENV:-false}" = "true" ]]; then
    buildTestRunnerImage
    runDockerCommand "SKIP_KIND_CREATE=true KIND_FORCE_CLEAN=false ./dev/reload.sh"
    return 0
  fi

  if [[ "${KIND}" = "true" ]]; then
    docker network create "${KIND_DOCKER_NETWORK}" || true
    _repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
    # shellcheck source=../../bin/build_utils
    . "${_repo_root}/bin/build_utils"
    retrieve_cyberark_ca_cert || true
  fi

  buildTestRunnerImage

  if [[ "${CONJUR_DEPLOYMENT}" != "cloud" ]]; then
    deployConjur
  fi

  # After Conjur is deployed, prevent subsequent commands from recreating the cluster
  if [[ "${KIND}" = "true" ]]; then
    export KIND_FORCE_CLEAN=false
  fi
  
  deployTest
}

buildTestRunnerImage() {
  pushd ..
  # Pre-fetch helm chart dependencies on the host so they are baked into the
  # image via COPY . . — the test container then finds them already present
  # and skips the network download, which may not be available in CI.
  helm repo add stakater https://stakater.github.io/stakater-charts 2>/dev/null || true
  helm repo update 2>/dev/null || true
  helm dependency build helm/secrets-provider || true
  docker build --tag "${TEST_RUNNER_IMAGE}:${CONJUR_NAMESPACE_NAME}" \
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
    
    # Load Conjur appliance image into Kind cluster if needed
    if [[ ${CONJUR_DEPLOYMENT} != oss && -n ${CONJUR_APPLIANCE_IMAGE:-} ]]; then
      source ./kind_helpers.sh
      docker tag ${CONJUR_APPLIANCE_IMAGE} conjur-appliance:${CONJUR_NAMESPACE_NAME}
      kind_load_image ${KIND_CLUSTER_NAME} ${CONJUR_APPLIANCE_IMAGE}
      kind_load_image ${KIND_CLUSTER_NAME} conjur-appliance:${CONJUR_NAMESPACE_NAME}
    fi

    cd ./kubernetes-conjur-deploy-$UNIQUE_TEST_ID
    
    export KIND=true
    export KIND_CLUSTER_NAME=${cluster_name}
    export LOG_LEVEL=debug
    export TEST_PLATFORM=kubernetes
    export CONJUR_NAMESPACE_NAME=${CONJUR_NAMESPACE_NAME}

    $cmd || {
      ts=\$(date -u +%Y%m%dT%H%M%SZ)
      echo '--- Conjur KinD deployment failed: dumping diagnostics ---'
      echo 'timestamp='\${ts}
      echo 'cluster='\${KIND_CLUSTER_NAME}
      echo 'namespace='\${CONJUR_NAMESPACE_NAME}
      echo
      echo '--- All pods ---'
      kubectl get pods -A -o wide || true
      echo
      echo '--- Cluster warning/error events ---'
      kubectl get events -A --sort-by='.lastTimestamp' | grep -E 'Warning|BackOff|CrashLoopBackOff|Failed|Evicted|Error|deadline|timeout' || true
      echo
      if [[ -n \${CONJUR_NAMESPACE_NAME} ]] && kubectl get namespace \${CONJUR_NAMESPACE_NAME} >/dev/null 2>&1; then
        echo '--- Namespace events ('\${CONJUR_NAMESPACE_NAME}') ---'
        kubectl -n \${CONJUR_NAMESPACE_NAME} get events --sort-by='.lastTimestamp' || true
        echo
        echo '--- Namespace pods ('\${CONJUR_NAMESPACE_NAME}') ---'
        kubectl -n \${CONJUR_NAMESPACE_NAME} get pods -o wide || true
        echo
        echo '--- Namespace workloads ('\${CONJUR_NAMESPACE_NAME}') ---'
        kubectl -n \${CONJUR_NAMESPACE_NAME} get deploy,rs,sts,svc -o wide || true
        echo
        echo '--- Describe pods ('\${CONJUR_NAMESPACE_NAME}') ---'
        kubectl -n \${CONJUR_NAMESPACE_NAME} describe pods || true
        echo
        echo '--- Conjur pod logs ('\${CONJUR_NAMESPACE_NAME}') ---'
        kubectl -n \${CONJUR_NAMESPACE_NAME} logs -l app=conjur-oss --all-containers --tail=300 || true
        kubectl -n \${CONJUR_NAMESPACE_NAME} logs -l app=conjur-cluster --all-containers --tail=300 || true
        kubectl -n \${CONJUR_NAMESPACE_NAME} logs -l app=conjur-postgres --all-containers --tail=300 || true
        kubectl -n \${CONJUR_NAMESPACE_NAME} logs -l app=conjur-cli --all-containers --tail=300 || true
        echo
        echo '--- Previous Conjur pod logs ('\${CONJUR_NAMESPACE_NAME}') ---'
        kubectl -n \${CONJUR_NAMESPACE_NAME} logs -l app=conjur-oss --all-containers --previous --tail=300 || true
        kubectl -n \${CONJUR_NAMESPACE_NAME} logs -l app=conjur-cluster --all-containers --previous --tail=300 || true
        kubectl -n \${CONJUR_NAMESPACE_NAME} logs -l app=conjur-postgres --all-containers --previous --tail=300 || true
        kubectl -n \${CONJUR_NAMESPACE_NAME} logs -l app=conjur-cli --all-containers --previous --tail=300 || true
      fi
      exit 1
    }"
}

# Main deployment logic
deployConjur() {
  # Prepare Docker images
  # This is done outside of the container to avoid authentication errors when pulling from the internal registry
  # from inside the container
  if [[ "${CONJUR_DEPLOYMENT}" != "oss" && -n "${CONJUR_APPLIANCE_IMAGE:-}" ]]; then
    docker pull "${CONJUR_APPLIANCE_IMAGE}"
  fi

  git clone --single-branch --branch master \
      https://github.com/cyberark/kubernetes-conjur-deploy.git \
      "kubernetes-conjur-deploy-${UNIQUE_TEST_ID}" || true

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
  if [[ "${KIND}" = "true" ]]; then
    runDockerCommand "set -e
      ./run_with_summon.sh || {
        echo '--- KinD test execution failed: dumping diagnostics ---'
        kubectl get pods -A -o wide || true
        kubectl get events -A --sort-by='.lastTimestamp' | grep -E 'Warning|BackOff|CrashLoopBackOff|Failed|Evicted|Error|deadline|timeout' || true

        if [[ -n \${CONJUR_NAMESPACE_NAME:-} ]] && kubectl get namespace \${CONJUR_NAMESPACE_NAME} >/dev/null 2>&1; then
          echo '--- Conjur namespace diagnostics ('\${CONJUR_NAMESPACE_NAME}') ---'
          kubectl -n \${CONJUR_NAMESPACE_NAME} get pods -o wide || true
          kubectl -n \${CONJUR_NAMESPACE_NAME} get events --sort-by='.lastTimestamp' || true
          kubectl -n \${CONJUR_NAMESPACE_NAME} logs -l app=conjur-oss --all-containers --tail=300 || true
          kubectl -n \${CONJUR_NAMESPACE_NAME} logs -l app=conjur-cluster --all-containers --tail=300 || true
          kubectl -n \${CONJUR_NAMESPACE_NAME} logs -l app=conjur-cli --all-containers --tail=300 || true
        fi

        if [[ -n \${APP_NAMESPACE_NAME:-} ]] && kubectl get namespace \${APP_NAMESPACE_NAME} >/dev/null 2>&1; then
          echo '--- App namespace diagnostics ('\${APP_NAMESPACE_NAME}') ---'
          kubectl -n \${APP_NAMESPACE_NAME} get pods -o wide || true
          kubectl -n \${APP_NAMESPACE_NAME} get events --sort-by='.lastTimestamp' || true
          kubectl -n \${APP_NAMESPACE_NAME} logs -l app=secrets-provider --all-containers --tail=300 || true
          kubectl -n \${APP_NAMESPACE_NAME} logs -l app=secrets-provider --all-containers --previous --tail=300 || true
          kubectl -n \${APP_NAMESPACE_NAME} logs -l app=test-env --all-containers --tail=300 || true
          kubectl -n \${APP_NAMESPACE_NAME} logs -l app.kubernetes.io/name=reloader --all-containers --tail=300 || true
          kubectl -n \${APP_NAMESPACE_NAME} logs -l app.kubernetes.io/name=reloader --all-containers --previous --tail=300 || true
        fi
        exit 1
      }
    "
  else
    runDockerCommand "./run_with_summon.sh"
  fi
}

main

if [[ "${DEV}" = "true" && "${RELOAD_ENV:-false}" != "true" ]]; then
  trap - EXIT
  announce "DEV=true: leaving KinD and Conjur in place; attaching to test runner (kubectl uses KUBECONFIG from entrypoint). Exit the shell when done."
  export SKIP_KIND_CREATE=true
  export KIND_FORCE_CLEAN=false
  runDockerInteractiveCommand
fi
