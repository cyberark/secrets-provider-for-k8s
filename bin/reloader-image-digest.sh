#!/usr/bin/env bash
# Maintainer tooling: print, verify, or sync Reloader image digest fields in helm/secrets-provider/values.yaml.
# Not required for installing the published chart—customers set values/overlays and pull from their registry.
#
# "sync" overwrites reloader.image.digest and reloader.imageDigestReference.pullReference using the
# locked sub-chart image tag, reloader.image.repository, and an optional reloader.image.tag override.
#
# Prerequisites: helm dependency build (charts/reloader-*.tgz). Needs Docker buildx for registry access.
# Run from repo root: ./bin/reloader-image-digest.sh [print|verify|sync]
#
set -euo pipefail

chart_dir="$(cd "$(dirname "$0")/../helm/secrets-provider" && pwd)"
values_file="${chart_dir}/values.yaml"
mode="${1:-print}"

shopt -s nullglob
tgz_list=("${chart_dir}/charts/reloader-"*.tgz)
shopt -u nullglob
if [[ ${#tgz_list[@]} -eq 0 ]]; then
  echo "No charts/reloader-*.tgz — run: helm dependency build (from helm/secrets-provider)" >&2
  exit 1
fi
tgz=$(printf '%s\n' "${tgz_list[@]}" | sort -V | tail -1)

subchart_tag=$(tar -xOf "${tgz}" reloader/values.yaml | grep -E '^[[:space:]]+tag:' | head -1 | awk '{print $2}')
if [[ -z "${subchart_tag}" ]]; then
  echo "Could not read image tag from ${tgz}" >&2
  exit 1
fi

repo=$(sed -n '/^reloader:/,$p' "${values_file}" | grep -E '^[[:space:]]+repository:' | head -1 | awk '{print $2}')
if [[ -z "${repo}" ]]; then
  echo "Could not read reloader.image.repository from values.yaml" >&2
  exit 1
fi

values_tag=$(sed -n '/^reloader:/,$p' "${values_file}" | grep -E '^[[:space:]]+tag:' | head -1 | awk '{print $2}' || true)
if [[ -n "${values_tag}" && "${values_tag}" != "${subchart_tag}" ]]; then
  echo "Note: reloader.image.tag (${values_tag}) overrides subchart default (${subchart_tag})." >&2
  effective_tag="${values_tag}"
else
  effective_tag="${subchart_tag}"
fi

# registry/repository:tag — docker.io prefix for short names (stakater/reloader)
canonical_pull_ref() {
  local r="$1"
  local t="$2"
  if [[ "${r}" =~ ^[a-zA-Z0-9._-]+\.[a-zA-Z0-9._-]+/ ]] || [[ "${r}" == localhost/* ]]; then
    echo "${r}:${t}"
  else
    echo "docker.io/${r}:${t}"
  fi
}

pull_ref=$(
  sed -n '/^reloader:/,$p' "${values_file}" |
    grep -E '^[[:space:]]+pullReference:' |
    head -1 |
    sed -E 's/^[[:space:]]+pullReference:[[:space:]]*//' |
    tr -d "\"'" |
    xargs
)

sync_ref=$(canonical_pull_ref "${repo}" "${effective_tag}")

if [[ "${mode}" == "sync" ]]; then
  ref="${sync_ref}"
else
  if [[ -n "${pull_ref}" ]]; then
    ref="${pull_ref}"
  else
    ref="${sync_ref}"
  fi
fi

registry_digest_buildx() {
  docker buildx imagetools inspect "${ref}" 2>/dev/null | awk '
    BEGIN { found=0 }
    /^Digest:/ { print $2; found=1; exit }
    END { exit !found }
  '
}

registry_digest() {
  local attempts=3 delay=5 count=1 digest

  while true; do
    if digest=$(registry_digest_buildx); then
      echo "${digest}"
      return 0
    fi
    if [[ "${count}" -ge "${attempts}" ]]; then
      return 1
    fi
    echo "docker buildx imagetools inspect failed (attempt ${count}/${attempts})" >&2
    echo "Retrying in ${delay}s..." >&2
    sleep "${delay}"
    count=$((count + 1))
    delay=$((delay * 2))
  done
}

sync_values() {
  local new_digest="$1"
  local new_pref="$2"
  local tmp
  tmp=$(mktemp)
  awk -v dig="${new_digest}" -v pref="${new_pref}" '
    /^[[:space:]]+digest:/ {
      sub(/digest:.*/, "digest: " dig)
    }
    /^[[:space:]]+pullReference:/ {
      sub(/pullReference:.*/, "pullReference: " pref)
    }
    { print }
  ' "${values_file}" > "${tmp}"
  mv "${tmp}" "${values_file}"
  echo "Updated ${values_file}: digest=${new_digest}, pullReference=${new_pref}"
}

case "${mode}" in
  print)
    echo "Locked sub-chart default tag: ${subchart_tag}"
    if [[ -n "${pull_ref}" ]]; then
      echo "Using imageDigestReference.pullReference: ${ref}"
    else
      echo "Derived reference: ${ref}"
    fi
    echo "Multi-arch digest (reloader.image.digest):"
    if ! d=$(registry_digest); then
      echo "docker buildx imagetools inspect failed — install Docker / buildx." >&2
      exit 1
    fi
    echo "${d}"
    ;;
  verify)
    expected=$(sed -n '/^reloader:/,$p' "${values_file}" | grep -E '^[[:space:]]+digest:' | head -1 | awk '{print $2}')
    if [[ -z "${expected}" ]]; then
      echo "reloader.image.digest missing in values.yaml" >&2
      exit 1
    fi
    if ! actual=$(registry_digest); then
      echo "docker buildx imagetools inspect failed — cannot verify (offline?)" >&2
      exit 2
    fi
    if [[ "${actual}" != "${expected}" ]]; then
      echo "Digest mismatch for ${ref}" >&2
      echo "  values.yaml: ${expected}" >&2
      echo "  registry:    ${actual}" >&2
      echo "Run: ( cd helm/secrets-provider && helm dependency update ) && ./bin/reloader-image-digest.sh sync" >&2
      exit 1
    fi
    echo "OK: reloader.image.digest matches registry for ${ref}"
    ;;
  sync)
    ref="${sync_ref}"
    echo "Resolving digest for ${ref} (repository + tag from locked chart or reloader.image.tag override)..."
    if ! new_d=$(registry_digest); then
      echo "docker buildx imagetools inspect failed — install Docker / buildx and network access." >&2
      exit 1
    fi
    sync_values "${new_d}" "${sync_ref}"
    ;;
  *)
    echo "Usage: $0 [print|verify|sync]" >&2
    exit 1
    ;;
esac
