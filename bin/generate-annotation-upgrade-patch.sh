#!/usr/bin/env bash

set -eo pipefail

readonly SP_ENV_VARS=(
  "CONJUR_AUTHN_LOGIN"
  "CONTAINER_MODE"
  "SECRETS_DESTINATION"
  "K8S_SECRETS"
  "RETRY_COUNT_LIMIT"
  "RETRY_INTERVAL_SEC"
  "DEBUG"
)

readonly SP_ANNOTS=(
  "conjur.org/authn-identity"
  "conjur.org/container-mode"
  "conjur.org/secrets-destination"
  "conjur.org/k8s-secrets"
  "conjur.org/retry-count-limit"
  "conjur.org/retry-interval-sec"
  "conjur.org/debug-logging"
)

DEPLOYMENT_NAME=''
NAMESPACE=''
PUSH_TO_FILE='false'

function print_help() {
  cat << EOF 1>&2
Generates a JSON patch containing operations to upgrade a deployment using
Secrets Provider to use annotations for configuration instead of
environment variables. The patch can be output to a file:

  $0 --push-to-file \$DEPLOYMENT_NAME | tee patch.json | jq

Test patch against a live deployment:

  kubectl patch deployment \$DEPLOYMENT_NAME --type json --patch-file patch.json --dry-run=server

Preview the new deployment:

  kubectl patch deployment \$DEPLOYMENT_NAME --type json --patch-file patch.json --dry-run=server --output yaml

Apply patch:

  kubectl patch deployment \$DEPLOYMENT_NAME --type json --patch-file patch.json

Usage: $0 [options] DEPLOYMENT_NAME:

    -p, --push-to-file        Upgrade to push-to-file mode
    -n, --namespace ''        If present, the namespace scope for this CLI request
                              All other selections are rejected
    -h, --help                Show this help message
EOF
  exit
}

function parse_args() {
  while true; do
    case "$1" in
      -p|--push-to-file )
        PUSH_TO_FILE='true'
        shift
        ;;
      -n|--namespace )
        shift
        NAMESPACE="$1"
        shift
        ;;
      -h|--help )
        print_help
        exit 0
        ;;
      * )
        if [[ -z "$1" ]] || [[ "${1:0:1}" == '-' ]]; then
          print_help
          exit 0
        else
          DEPLOYMENT_NAME="$1"
          break
        fi
        ;;
    esac
  done
}

function get_sp_init_container() {
  local deployment_manifest_json="$1"

  echo "${deployment_manifest_json}" | \
    jq \
      '
        .spec.template.spec.initContainers |
        map(select(.image | contains("secrets-provider-for-k8s"))) |
        to_entries |
        first(.[])
      '
}

function get_sp_init_container_env() {
  local sp_init_container="$1"

  echo "${sp_init_container}" | \
    jq \
      --arg SP_ENV_VARS "$( IFS=$','; echo "${SP_ENV_VARS[*]}" )" \
      '
        .value.env |
        to_entries |
        map(select(.value.name | inside($SP_ENV_VARS)))
      '
}

function new_annots_from_env() {
  local sp_env="$1"
  local sp_annots
  # Replace env var names with corresponding annotation names
  sp_annots="$(echo "${sp_env}" | jq 'map(.value)')"

  for i in "${!SP_ENV_VARS[@]}"; do
    local env_var="${SP_ENV_VARS[i]}"
    local annot="${SP_ANNOTS[i]}"

    sp_annots="$(echo "${sp_annots}" | \
      jq \
        --arg env_var "${env_var}" \
        --arg annot "${annot}" \
        'map(select(.name == $env_var) |= { key: $annot, value: .value })'
    )"
  done

  # Transform annotations json for patch
  echo "${sp_annots}" | jq 'from_entries'
}

function get_k8s_secrets_from_annots() {
  local sp_annots="$1"

  # Comma separated values, spaces not allowed
  echo "${sp_annots}" | \
    jq \
      --raw-output \
      '
        ."conjur.org/k8s-secrets" // "" |
        gsub("\\s+"; "") |
        split(",") |
        @sh
      '
}

function append_secrets_destination_annot() {
  local sp_annots="$1"

  echo "${sp_annots}" | \
    jq \
      '
        . +
        { "conjur.org/secrets-destination": "file" }
      '
}

function append_push_to_file_annots() {
  local sp_annots="$1"
  local k8s_secrets="$2"

  for k8s_secret in ${k8s_secrets[*]}; do
    local k8s_secret_manifest_json
    local conjur_secret
    # Remove wrapping single quotes
    k8s_secret="$(echo "${k8s_secret}" | sed "s/'//g")"

    k8s_secret_manifest_json="$(kubectl --namespace "${NAMESPACE}" get secret "${k8s_secret}" --output json)"
    if [[ "$?" -ne 0 ]]; then
      exit 1
    fi

    # Extract conjur-map from secret
    conjur_secret="$(echo "${k8s_secret_manifest_json}" | jq --raw-output '.data."conjur-map" // ""' | base64 --decode)"
    if [[ -z "${conjur_secret}" ]]; then
      echo "\"conjur-map\" not found in secret \"${k8s_secret}\"" 1>&2
      exit 1
    fi

    # Append push-to-file annotations
    sp_annots="$(echo "${sp_annots}" | \
      jq \
        --arg k8s_secret "${k8s_secret}" \
        --arg conjur_secret "${conjur_secret}" \
        '
          . +
          { ("conjur.org/conjur-secrets." + $k8s_secret): ("- " + $conjur_secret | gsub("\n"; "\n- ")) } +
          { ("conjur.org/secret-file-format." + $k8s_secret): "dotenv" }
        '
    )"
  done

  echo "${sp_annots}"
}

function new_patch_from_annots() {
  local sp_annots="$1"

  if [[ "${sp_annots}" == "{}" ]] || [[ -z "${sp_annots}" ]]; then
    echo '[]'
    return
  fi

  echo "${sp_annots}" | \
    jq \
      '
        [
          {
            "op": "add",
            "path": "/spec/template/metadata/annotations",
            "value": .
          }
        ]
      '
}

function append_sp_init_container_remove_ops_to_patch() {
  local patch="$1"
  local sp_init_container_idx="$2"
  local sp_env_var_indices="$3"

  for sp_env_var_idx in ${sp_env_var_indices[*]}; do
    patch="$(echo "${patch}" | \
      jq \
        --arg sp_init_container_idx "${sp_init_container_idx}" \
        --arg sp_env_var_idx "${sp_env_var_idx}" \
        '
          . +
          [
            {
              "op": "remove",
              "path": ("/spec/template/spec/initContainers/" + $sp_init_container_idx + "/env/" + $sp_env_var_idx)
            }
          ]
        '
    )"
  done

  echo "${patch}"
}

# Example output JSON:
# [
#   {
#     "containerIdx": 0,
#     "image": "docker.io/cyberark/demo-app:latest",
#     "hasConjurSecretsVolumeMounts": null,
#     "k8sCommand": null,
#     "secrets": [
#       "test-app-secrets-provider-init-secret"
#     ],
#     "spEnvVarIdxList": [
#       2,
#       1,
#       0
#     ]
#   }
# ]
function get_app_containers_json() {
  local deployment_manifest_json="$1" 
  local k8s_secrets="$2"

  # Reverse order to prevent indices from changing during 'remove' patch operations
  echo "${deployment_manifest_json}" | \
    jq \
      --arg k8s_secrets "$( IFS=$','; echo "${k8s_secrets[*]}" )" \
      '
        .spec.template.spec.containers |
        to_entries |
        map(
          {
            containerIdx: .key,
            image: .value.image,
            k8sCommand: ((.value.command // []) + (.value.args // [])) | join(" "),
            hasConjurSecretsVolumeMounts: .value.volumeMounts,
            secrets: (.value.env // [] | to_entries)
          }
        ) |
        map(. + { secrets: .secrets | map({ key: .key, value: .value.valueFrom.secretKeyRef.name }) }) |
        map(. + { secrets: .secrets | map(select(.value | inside($k8s_secrets))) }) |
        map(select(.secrets | length > 0)) |
        map(. + { spEnvVarIdxList: (.secrets | map(.key) | reverse) }) |
        map(. + { secrets: [.secrets | .[].value] | unique }) |
        map(. + { hasConjurSecretsVolumeMounts: (try (.hasConjurSecretsVolumeMounts | map(select(.name == "conjur-secrets")) | length > 0) catch null) }) |
        reverse
      '
}

function append_app_container_remove_ops_to_patch() {
  local patch="$1"
  local app_containers_json="$2"

  echo "${patch}" | \
    jq \
      --argjson app_containers_json "${app_containers_json}" \
      '
        . +
        (
          $app_containers_json |
          map(
            [
              {
                "op": "remove",
                "path": ("/spec/template/spec/containers/" + (.containerIdx | tostring) + "/env/" + (.spEnvVarIdxList | .[] | tostring))
              }
            ]
          ) |
          flatten
        )
      '
}

function deployment_has_volume() {
  local deployment_manifest_json="$1"
  local volume_name="$2"

  echo "${deployment_manifest_json}" | \
    jq \
      --arg volume_name "${volume_name}" \
      '
        .spec.template.spec.volumes // [] |
        map(select(.name == $volume_name)) |
        length > 0
      '
}

function sp_init_container_has_volume_mount() {
  local deployment_manifest_json="$1"
  local sp_init_container_idx="$2"
  local volume_mount_name="$3"

  echo "${deployment_manifest_json}" | \
    jq \
      --arg sp_init_container_idx "${sp_init_container_idx}" \
      --arg volume_mount_name "${volume_mount_name}" \
      '
        .spec.template.spec.initContainers[$sp_init_container_idx | tonumber].volumeMounts // [] |
        map(select(.name == $volume_mount_name)) |
        length > 0
      '
}

function check_append_empty_volume_list_to_patch() {
  local patch="$1"
  local deployment_manifest_json="$2"
  local no_volumes

  no_volumes="$(echo "${deployment_manifest_json}" | jq '.spec.template.spec.volumes == null')"
  if [[ "$no_volumes" == "false" ]]; then
    echo "${patch}"
    return
  fi

  echo "${patch}" | \
    jq \
      '
        . +
        [
          {
            "op": "add",
            "path": "/spec/template/spec/volumes",
            "value": []
          }
        ]
      '
}

function check_append_sp_init_container_empty_volume_mount_list_to_patch() {
  local patch="$1"
  local deployment_manifest_json="$2"
  local sp_init_container_idx="$3"
  local no_volume_mounts

  no_volume_mounts="$(echo "${deployment_manifest_json}" | \
    jq \
      --arg sp_init_container_idx "${sp_init_container_idx}" \
      '
        .spec.template.spec.initContainers[$sp_init_container_idx | tonumber].volumeMounts == null
      '
  )"

  if [[ "$no_volume_mounts" == "false" ]]; then
    echo "${patch}"
    return
  fi

  echo "${patch}" | \
    jq \
      --arg sp_init_container_idx "${sp_init_container_idx}" \
      '
        . +
        [
          {
            "op": "add",
            "path": ("/spec/template/spec/initContainers/" + $sp_init_container_idx + "/volumeMounts"),
            "value": []
          }
        ]
      '
}

function check_append_app_containers_empty_volume_mount_lists_to_patch() {
  local patch="$1"
  local app_containers_json="$2"
  
  echo "${patch}" | \
    jq \
      --argjson app_containers_json "${app_containers_json}" \
      '
        . +
        (
          $app_containers_json |
          map(select(.hasConjurSecretsVolumeMounts == null)) |
          [
            {
              "op": "add",
              "path": ("/spec/template/spec/containers/" + (.[].containerIdx | tostring) + "/volumeMounts"),
              "value": []
            }
          ]
        )
      '
}

function append_podinfo_volume_to_patch() {
  local patch="$1"

  echo "${patch}" | \
    jq \
      '
        . +
        [
          {
            "op": "add",
            "path": "/spec/template/spec/volumes/-",
            "value": {
              "name": "podinfo",
              "downwardAPI": {
                "items": [
                  {
                    "path": "annotations",
                    "fieldRef": {
                      "fieldPath": "metadata.annotations"
                    }
                  }
                ]
              }
            }
          }
        ]
      '
}

function append_podinfo_volume_mount_to_patch() {
  local patch="$1"
  local sp_init_container_idx="$2"

  echo "${patch}" | \
    jq \
      --arg sp_init_container_idx "${sp_init_container_idx}" \
      '
        . +
        [
          {
            "op": "add",
            "path": ("/spec/template/spec/initContainers/" + $sp_init_container_idx + "/volumeMounts/-"),
            "value": {
              "mountPath": "/conjur/podinfo",
              "name": "podinfo"
            }
          }
        ]
      '
}

function append_conjur_secrets_volume_to_patch() {
  local patch="$1"

  echo "${patch}" | \
    jq \
      '
        . +
        [
          {
            "op": "add",
            "path": "/spec/template/spec/volumes/-",
            "value": {
              "name": "conjur-secrets",
              "emptyDir": {
                "medium": "Memory"
              }
            }
          }
        ]
      '
}

function append_sp_init_container_conjur_secrets_volume_mount_to_patch() {
  local patch="$1"
  local sp_init_container_idx="$2"

  echo "${patch}" | \
    jq \
      --arg sp_init_container_idx "${sp_init_container_idx}" \
      '
        . +
        [
          {
            "op": "add",
            "path": ("/spec/template/spec/initContainers/" + $sp_init_container_idx + "/volumeMounts/-"),
            "value": {
              "mountPath": "/conjur/secrets",
              "name": "conjur-secrets"
            }
          }
        ]
      '
}

function append_app_containers_conjur_secrets_volume_mounts_to_patch() {
  local patch="$1"
  local app_containers_json="$2"

  echo "${patch}" | \
    jq \
      --argjson app_containers_json "${app_containers_json}" \
      '
        . +
        (
          $app_containers_json |
          map(select((.hasConjurSecretsVolumeMounts // false) == false)) |
          [
            {
              "op": "add",
              "path": ("/spec/template/spec/containers/" + (.[].containerIdx | tostring) + "/volumeMounts/-"),
              "value": {
                "mountPath": "/conjur/secrets",
                "name": "conjur-secrets"
              }
            }
          ]
        )
      '
}

function append_app_container_command_replace_ops_to_patch() {
  local patch="$1"
  local app_containers_json="$2"
  local group_name="test-app-secrets-provider-init"
  local app_containers_len

  # Get env var count -1 for bash for loop
  app_containers_len="$(echo "${app_containers_json}" | jq --raw-output 'length - 1')"

  if [[ "${app_containers_len}" == "-1" ]]; then
    echo "${patch}"
    return
  fi

  for i in $(seq 0 "${app_containers_len}"); do
    local container_idx
    local k8s_cmd
    local cmd

    container_idx="$(echo "${app_containers_json}" | jq --arg i "${i}" '(.[$i | tonumber]).containerIdx')"
    k8s_cmd="$(echo "${app_containers_json}" | jq --arg i "${i}" '(.[$i | tonumber]).k8sCommand')"
    cmd="${k8s_cmd}"

    if [[ -z "${cmd}" ]]; then
      local image
      local image_cmd
      
      image="$(echo "${app_containers_json}" | jq --arg i "${i}" --raw-output '.[$i | tonumber].image')"
      image_cmd="$(docker image inspect "$image" | jq 'first(.[]) // {} | .Config | (.Cmd // []) + (.Entrypoint // []) | join(" ")')"
      cmd="${image_cmd}"

      if [[ -z "${cmd}" ]]; then
        continue
      fi
    fi
    
    if [[ -n "${image_cmd}" ]]; then
      patch="$(echo "${patch}" | \
        jq \
          --arg container_idx "${container_idx}" \
          --arg group_name "${group_name}" \
          --arg cmd "${cmd}" \
          '
            . +
            [
              {
                "op": "replace",
                "path": ("/spec/template/spec/containers/" + $container_idx + "/command"),
                "value": [ "/bin/sh" ]
              },
              {
                "op": "replace",
                "path": ("/spec/template/spec/containers/" + $container_idx + "/args"),
                "value": [ "-c", (". /conjur/secrets/" + $group_name + "; " + $cmd) ]
              }
            ]
          '
      )"
    fi
  done

  echo "${patch}"
}

function main() {
  parse_args "$@"

  local deployment_manifest_json
  local sp_init_container
  local sp_init_container_idx
  local sp_env
  local sp_env_var_indices
  local sp_annots
  local k8s_secrets
  local patch
  local app_containers_json

  deployment_manifest_json="$(kubectl --namespace "${NAMESPACE}" get deployment "${DEPLOYMENT_NAME}" --output json)"
  if [[ "$?" -ne 0 ]]; then
    exit 1
  fi

  # Extract the SP init container
  sp_init_container="$(get_sp_init_container "${deployment_manifest_json}")"
  if [[ -z "${sp_init_container}" ]]; then
    echo "Could not find Secrets Provider init container" 1>&2
    exit 1
  fi

  # Store the index of the SP container, used for remove patch later
  sp_init_container_idx="$(echo "${sp_init_container}" | jq '.key')"

  # Extract SP env vars
  sp_env="$(get_sp_init_container_env "${sp_init_container}")"

  # Store the indices of SP env vars in a bash array, used for remove patch later
  # Reverse sort env vars indices to prevent indices from changing during 'remove' patch operations
  sp_env_var_indices=($(echo "${sp_env}" | jq --raw-output 'map(.key) | reverse | @sh'))

  sp_annots="$(new_annots_from_env "${sp_env}")"

  if [[ "${PUSH_TO_FILE}" == 'true' ]]; then
    # Extract k8s secret names from annotation to bash array
    k8s_secrets=($(get_k8s_secrets_from_annots "${sp_annots}"))

    # Exit if k8s_secrets array is empty
    if [[ "${#k8s_secrets[@]}" -eq 0 ]]; then
      # If k8s secrets list is not in new annotations, check existing annotations
      k8s_secrets=($(get_k8s_secrets_from_annots "$(echo "${deployment_manifest_json}" | jq '.spec.template.metadata.annotations')"))

      if [[ "${#k8s_secrets[@]}" -eq 0 ]]; then
        echo "Missing K8S_SECRETS environment variable" 1>&2
        exit 1
      fi
    fi

    # Append secrets destination annotation
    # Replaces instead if key exists
    sp_annots="$(append_secrets_destination_annot "${sp_annots}")"

    # Append push-to-file annotations
    sp_annots="$(append_push_to_file_annots "${sp_annots}" "${k8s_secrets[*]}")"
  fi

  # Create initial patch with annotation 'add' operation
  patch="$(new_patch_from_annots "${sp_annots}")"

  # Update patch with env var 'remove' operations
  # Remove env vars from init containers
  patch="$(append_sp_init_container_remove_ops_to_patch "${patch}" "${sp_init_container_idx}" "${sp_env_var_indices[*]}")"

  if [[ "${PUSH_TO_FILE}" == 'true' ]]; then
    # Extract all application containers with env vars referencing a secret provided by SP
    app_containers_json="$(get_app_containers_json "${deployment_manifest_json}" "${k8s_secrets[*]}")"

    # Remove env vars from app containers
    patch="$(append_app_container_remove_ops_to_patch "${patch}" "${app_containers_json}")"

    # If necessary initialize volume list to []
    patch="$(check_append_empty_volume_list_to_patch "${patch}" "${deployment_manifest_json}")"

    if [[ $(deployment_has_volume "${deployment_manifest_json}" 'podinfo') == "false" ]]; then
      patch="$(append_podinfo_volume_to_patch "${patch}")"
    fi

    if [[ $(deployment_has_volume "${deployment_manifest_json}" 'conjur-secrets') == "false" ]]; then
      patch="$(append_conjur_secrets_volume_to_patch "${patch}")"
    fi

    # If necessary initialize SP init container volume mount list to []
    patch="$(check_append_sp_init_container_empty_volume_mount_list_to_patch "${patch}" "${deployment_manifest_json}" "${sp_init_container_idx}")"

    if [[ $(sp_init_container_has_volume_mount "${deployment_manifest_json}" "${sp_init_container_idx}" 'podinfo') == "false" ]]; then
      patch="$(append_podinfo_volume_mount_to_patch "${patch}" "${sp_init_container_idx}")"
    fi

    if [[ $(sp_init_container_has_volume_mount "${deployment_manifest_json}" "${sp_init_container_idx}" 'conjur-secrets') == "false" ]]; then
      patch="$(append_sp_init_container_conjur_secrets_volume_mount_to_patch "${patch}" "${sp_init_container_idx}")"
    fi

    # If necessary initialize app containers' volume mount lists to []
    patch="$(check_append_app_containers_empty_volume_mount_lists_to_patch "${patch}" "${app_containers_json}")"

    patch="$(append_app_containers_conjur_secrets_volume_mounts_to_patch "${patch}" "${app_containers_json}")"

    patch="$(append_app_container_command_replace_ops_to_patch "${patch}" "${app_containers_json}")"
  fi

  echo "${patch}" | jq
}

main "$@"
