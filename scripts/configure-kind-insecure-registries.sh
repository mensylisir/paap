#!/usr/bin/env bash
set -euo pipefail

KIND_CLUSTER="${KIND_CLUSTER:-}"
CONTAINER_CLI="${CONTAINER_CLI:-docker}"
KUBECTL_BIN="${KUBECTL_BIN:-kubectl}"

usage() {
  cat <<'USAGE'
Usage: scripts/configure-kind-insecure-registries.sh [host:port ...]

Discovers registry/Harbor Services in the current Kubernetes context and writes
containerd hosts.toml entries into every node of the selected kind cluster.

Environment:
  KIND_CLUSTER    kind cluster name or full kind context suffix, default kind
  CONTAINER_CLI   docker or podman-compatible CLI, default docker
  KUBECTL_BIN     kubectl binary, default kubectl

Extra host:port arguments are added to the discovered list.
USAGE
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if [[ -z "$KIND_CLUSTER" ]]; then
  current_context="$("$KUBECTL_BIN" config current-context 2>/dev/null || true)"
  if [[ "$current_context" == kind-* ]]; then
    KIND_CLUSTER="${current_context#kind-}"
  else
    KIND_CLUSTER="kind"
  fi
fi

kind_label="${KIND_CLUSTER#kind-}"
node_containers="$("$CONTAINER_CLI" ps \
  --filter "label=io.x-k8s.kind.cluster=${kind_label}" \
  --format '{{.Names}}' | awk 'NF')"

if [[ -z "$node_containers" ]]; then
  echo "ERROR: no kind node containers found for cluster ${kind_label}" >&2
  exit 1
fi

node_host="$("$KUBECTL_BIN" get nodes -o jsonpath='{range .items[*]}{range .status.addresses[?(@.type=="InternalIP")]}{.address}{"\n"}{end}{end}' 2>/dev/null | awk 'NF { print; exit }' || true)"

tmp_hosts="$(mktemp)"
tmp_mirrors="$(mktemp)"
trap 'rm -f "$tmp_hosts" "$tmp_mirrors"' EXIT

"$KUBECTL_BIN" get svc -A -o jsonpath='{range .items[*]}{.metadata.namespace}{"\t"}{.metadata.name}{"\t"}{.spec.clusterIP}{"\t"}{range .spec.ports[*]}{.port}{":"}{.nodePort}{","}{end}{"\n"}{end}' \
  | awk -F '\t' '
      BEGIN { IGNORECASE = 1 }
      $2 ~ /(registry|harbor)/ && $3 != "" && $3 != "None" {
        split($4, ports, ",")
        for (i in ports) {
          if (ports[i] == "") continue
          split(ports[i], pair, ":")
          port = pair[1]
          if (port ~ /^[0-9]+$/) print $3 ":" port
        }
      }
    ' >> "$tmp_hosts"

"$KUBECTL_BIN" get svc -A -o jsonpath='{range .items[*]}{.metadata.namespace}{"\t"}{.metadata.name}{"\t"}{.spec.clusterIP}{"\t"}{range .spec.ports[*]}{.port}{","}{end}{"\n"}{end}' \
  | awk -F '\t' '
      BEGIN { IGNORECASE = 1 }
      $2 ~ /(registry|harbor)/ && $3 != "" && $3 != "None" {
        split($4, ports, ",")
        port = ""
        for (i in ports) {
          if (ports[i] == "") continue
          if (ports[i] == 5000 || ports[i] == 80 || ports[i] == 443) {
            port = ports[i]
            break
          }
          if (port == "") port = ports[i]
        }
        if (port != "") print $1 "\t" $2 "\t" $3 ":" port
      }
    ' | while IFS=$'\t' read -r namespace name target; do
      [[ -z "$namespace" || -z "$name" || -z "$target" ]] && continue
      "$KUBECTL_BIN" -n "$namespace" get ingress -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{range .spec.rules[*]}{.host}{"\n"}{end}{end}' 2>/dev/null \
        | awk -F '\t' -v svc="$name" -v target="$target" '
            BEGIN { IGNORECASE = 1 }
            $1 == svc && $2 != "" { print $2 "\t" target }
          '
    done >> "$tmp_mirrors"

if [[ -n "$node_host" ]]; then
  "$KUBECTL_BIN" get svc -A -o jsonpath='{range .items[*]}{.metadata.namespace}{"\t"}{.metadata.name}{"\t"}{range .spec.ports[*]}{.nodePort}{","}{end}{"\n"}{end}' \
    | awk -F '\t' -v host="$node_host" '
        BEGIN { IGNORECASE = 1 }
        $2 ~ /(registry|harbor)/ {
          split($3, ports, ",")
          for (i in ports) {
            if (ports[i] ~ /^[0-9]+$/ && ports[i] > 0) print host ":" ports[i]
          }
        }
      ' >> "$tmp_hosts"
fi

for extra in "$@"; do
  if [[ "$extra" == *:* ]]; then
    printf '%s\n' "$extra" >> "$tmp_hosts"
  fi
done

mapfile -t hosts < <(sort -u "$tmp_hosts" | awk 'NF')
mapfile -t mirrors < <(sort -u "$tmp_mirrors" | awk 'NF')
if [[ "${#hosts[@]}" -eq 0 && "${#mirrors[@]}" -eq 0 ]]; then
  echo "No registry/Harbor hosts discovered."
  exit 0
fi

echo "Configuring insecure registry hosts for kind cluster ${kind_label}:"
if [[ "${#hosts[@]}" -gt 0 ]]; then
  printf '  %s\n' "${hosts[@]}"
fi
if [[ "${#mirrors[@]}" -gt 0 ]]; then
  printf '  %s\n' "${mirrors[@]}" | awk -F '\t' '{ printf "  %s -> %s\n", $1, $2 }'
fi

for node in $node_containers; do
  echo "-> ${node}"
  for host in "${hosts[@]}"; do
    "$CONTAINER_CLI" exec "$node" sh -ceu '
      host="$1"
      dir="/etc/containerd/certs.d/${host}"
      mkdir -p "$dir"
      cat > "${dir}/hosts.toml" <<EOF
server = "http://${host}"

[host."http://${host}"]
  capabilities = ["pull", "resolve", "push"]
  skip_verify = true
EOF
    ' sh "$host"
  done
  for mirror in "${mirrors[@]}"; do
    registry_host="${mirror%%$'\t'*}"
    mirror_target="${mirror#*$'\t'}"
    "$CONTAINER_CLI" exec "$node" sh -ceu '
      registry_host="$1"
      mirror_target="$2"
      dir="/etc/containerd/certs.d/${registry_host}"
      mkdir -p "$dir"
      cat > "${dir}/hosts.toml" <<EOF
server = "https://${registry_host}"

[host."http://${mirror_target}"]
  capabilities = ["pull", "resolve", "push"]
  skip_verify = true
EOF
    ' sh "$registry_host" "$mirror_target"
  done
done

echo "containerd hosts.toml entries updated. New pulls will use HTTP for the listed registry hosts."
