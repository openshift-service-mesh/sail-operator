#!/bin/bash

# Copyright Istio Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Plan / verify / apply Red Hat helm chart swaps + Konflux placeholders for
# openshift-service-mesh/sail-operator release branches.
#
# Typical flow (from any checkout, including upstream main):
#   ./hack/add-rh-istio-charts.sh --discover
#   ./hack/add-rh-istio-charts.sh --discover --dry-run
#   ./hack/add-rh-istio-charts.sh --discover --create-pr   # open review PRs (no auto-merge)
#   ./hack/add-rh-istio-charts.sh --dry-run --branch release-3.4 --versions 1.30.8
#   ./hack/add-rh-istio-charts.sh --branch release-3.4 --versions 1.30.8
#
# Requires: gh, yq, curl, python3. Apply / --create-pr also need a clean OSSM
# checkout with pkg/istioversion/versions.ossm.yaml and ossm/values.yaml, plus
# git push permission. Created PRs are for human review (no auto-merge labels).

set -euo pipefail

REPO="${REPO:-openshift-service-mesh/sail-operator}"
ISTIO_RELEASE_REPO="${ISTIO_RELEASE_REPO:-openshift-service-mesh/istio-release}"
VERSIONS_YAML_FILE="${VERSIONS_YAML_FILE:-pkg/istioversion/versions.ossm.yaml}"
HELM_VALUES_FILE="${HELM_VALUES_FILE:-ossm/values.yaml}"
CHARTS=(base istiod gateway cni ztunnel)

MODE="apply" # apply | dry-run | verify-only
BRANCH=""
VERSIONS_CSV=""
FORCE_COMMIT=""
SKIP_PR_CHECK=false
FETCH_REMOTE_FILES=false
DISCOVER=false
MAKE_GEN=false
# When discover --dry-run finds versions that already have an open RH PR, skip them
# by default so we do not recreate in-flight work. Pass --include-open to plan them anyway.
INCLUDE_OPEN=false
CREATE_PR=false
# When discovering, consider the N newest release-X.Y lines.
# OSSM keeps older minors shipping (3.1/3.2/3.3/...), so default is wider than upstream n-2.
DISCOVER_ACTIVE_COUNT="${DISCOVER_ACTIVE_COUNT:-5}"
# Optional comma-separated override, e.g. release-3.1,release-3.2,release-3.3,release-3.4,release-3.5
DISCOVER_BRANCHES="${DISCOVER_BRANCHES:-}"
GIT_REMOTE="${GIT_REMOTE:-origin}"
PR_LABELS="${PR_LABELS:-}" # optional comma-separated labels (never auto-merge)

RED=$'\033[31m'
GRN=$'\033[32m'
YLW=$'\033[33m'
BLU=$'\033[34m'
RST=$'\033[0m'

# All human-readable logs go to stderr so command substitutions stay clean.
info() { printf '%s==>%s %s\n' "$BLU" "$RST" "$*" >&2; }
ok()   { printf '%sOK%s  %s\n' "$GRN" "$RST" "$*" >&2; }
warn() { printf '%sWARN%s %s\n' "$YLW" "$RST" "$*" >&2; }
err()  { printf '%sERR%s  %s\n' "$RED" "$RST" "$*" >&2; }
die()  { err "$*"; exit 1; }

usage() {
  cat <<'EOF'
Usage: hack/add-rh-istio-charts.sh [options]

Discover all ready RH chart swaps (run from anywhere, including main):
  ./hack/add-rh-istio-charts.sh --discover
  ./hack/add-rh-istio-charts.sh --discover --dry-run
  ./hack/add-rh-istio-charts.sh --discover --create-pr   # open PRs for human review

Single-branch:
  ./hack/add-rh-istio-charts.sh --dry-run --branch release-3.4 --versions 1.30.8
  ./hack/add-rh-istio-charts.sh --branch release-3.4 --versions 1.30.8 --create-pr

Options:
  --discover                    Scan active OSSM release-X.Y branches for versions that
                                still use upstream charts but have RH charts published.
                                Implies --fetch-remote. Prints a summary table.
                                Combine with --dry-run to plan every ready swap.
  --create-pr                   After apply (+ make gen), commit, push, and open a PR
                                against the release branch. Does NOT add auto-merge
                                labels — humans must review. Implies --make-gen.
  --branch release-X.Y          Target OSSM release branch (required unless --discover)
  --versions V[,V...]           Istio versions to convert (required unless --discover)
  --dry-run                     Verify + print planned edits; do not write files
  --verify-only                 Only run prerequisite / previous-PR checks
  --commit SHA                  Force istio-release commit (default: auto-detect per version)
  --repo owner/name             Sail Operator repo (default: openshift-service-mesh/sail-operator)
  --istio-release owner/name    istio-release repo (default: openshift-service-mesh/istio-release)
  --fetch-remote                Read versions/values from GitHub branch instead of local files
  --skip-pr-check               Do not fail if an open RH charts PR already covers the versions
  --make-gen                    After apply, run OSSM make gen
                                (VERSIONS_YAML_FILE=versions.ossm.yaml HELM_VALUES_FILE=ossm/values.yaml)
  --branches B[,B...]           With --discover, scan these release-X.Y branches instead of newest N
  --include-open                With --discover --dry-run, also plan versions that already have
                                an open RH charts PR (default: skip those)
  -h, --help                    Show this help

Env:
  DISCOVER_ACTIVE_COUNT         Newest N release-X.Y lines when --branches unset (default: 5)
  DISCOVER_BRANCHES             Comma-separated branch list (same as --branches)

Exit codes:
  0 success (or dry-run/verify/discover passed)
  1 verification or apply failure
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --discover) DISCOVER=true; FETCH_REMOTE_FILES=true; shift ;;
    --branch) BRANCH="${2:-}"; shift 2 ;;
    --branches) DISCOVER_BRANCHES="${2:-}"; shift 2 ;;
    --versions) VERSIONS_CSV="${2:-}"; shift 2 ;;
    --dry-run) MODE="dry-run"; shift ;;
    --verify-only) MODE="verify-only"; shift ;;
    --commit) FORCE_COMMIT="${2:-}"; shift 2 ;;
    --repo) REPO="${2:-}"; shift 2 ;;
    --istio-release) ISTIO_RELEASE_REPO="${2:-}"; shift 2 ;;
    --fetch-remote) FETCH_REMOTE_FILES=true; shift ;;
    --skip-pr-check) SKIP_PR_CHECK=true; shift ;;
    --make-gen) MAKE_GEN=true; shift ;;
    --include-open) INCLUDE_OPEN=true; shift ;;
    --create-pr) CREATE_PR=true; MAKE_GEN=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "Unknown argument: $1" ;;
  esac
done

need_cmd() { command -v "$1" >/dev/null 2>&1 || die "Missing required command: $1"; }
need_cmd gh
need_cmd yq
need_cmd curl
need_cmd python3
if $CREATE_PR && [[ "$MODE" != "dry-run" && "$MODE" != "verify-only" ]]; then
  need_cmd git
fi

TMPDIR_ROOT="$(mktemp -d)"
trap 'rm -rf "$TMPDIR_ROOT"' EXIT
RH_PR_CACHE_FILE="${TMPDIR_ROOT}/rh-prs.tsv"
declare -A MUST_GATHER_CACHE=()
VERSIONS=()

VERSIONS_PATH="$VERSIONS_YAML_FILE"
VALUES_PATH="$HELM_VALUES_FILE"

fetch_file() {
  local path="$1" dest="$2" ref="${3:-$BRANCH}"
  gh api "repos/${REPO}/contents/${path}?ref=${ref}" --jq .content | base64 -d >"$dest"
}

load_branch_files() {
  local ref="$1"
  MUST_GATHER_CACHE=()
  if $FETCH_REMOTE_FILES || [[ ! -f "$VERSIONS_YAML_FILE" || ! -f "$HELM_VALUES_FILE" ]]; then
    if [[ ! -f "$VERSIONS_YAML_FILE" || ! -f "$HELM_VALUES_FILE" ]]; then
      warn "Local ${VERSIONS_YAML_FILE} / ${HELM_VALUES_FILE} missing; fetching from ${REPO}@${ref}"
    else
      info "Fetching ${VERSIONS_YAML_FILE} and ${HELM_VALUES_FILE} from ${REPO}@${ref}"
    fi
    VERSIONS_PATH="${TMPDIR_ROOT}/versions-${ref}.ossm.yaml"
    VALUES_PATH="${TMPDIR_ROOT}/values-${ref}.yaml"
    fetch_file "$VERSIONS_YAML_FILE" "$VERSIONS_PATH" "$ref"
    fetch_file "$HELM_VALUES_FILE" "$VALUES_PATH" "$ref"
    if [[ "$MODE" == "apply" ]]; then
      die "Apply mode requires a local OSSM checkout with ${VERSIONS_YAML_FILE} and ${HELM_VALUES_FILE}. Re-run with --dry-run/--verify-only/--discover, or check out ${REPO}@${ref} first."
    fi
  else
    VERSIONS_PATH="$VERSIONS_YAML_FILE"
    VALUES_PATH="$HELM_VALUES_FILE"
  fi
}

underscore() { printf '%s' "${1//./_}"; }
istio_minor() { printf '%s' "${1%.*}"; }

ossm_minor_from_branch() {
  # release-3.4 -> 3_4
  local rest="${BRANCH#release-}"
  printf '%s' "${rest//./_}"
}

is_upstream_chart_url() {
  [[ "$1" == *"blob.istio.io"* || "$1" == *"istio-release.storage.googleapis.com"* ]]
}

is_rh_chart_url() {
  [[ "$1" == *"openshift-service-mesh/istio-release"* && "$1" == *"-redhat/helm/"* ]]
}

version_entry_exists() {
  local ver="$1"
  yq -e ".versions[] | select(.name == \"v${ver}\")" "$VERSIONS_PATH" >/dev/null 2>&1
}

chart_urls_for_version() {
  local ver="$1"
  yq -r ".versions[] | select(.name == \"v${ver}\") | .charts[]" "$VERSIONS_PATH"
}

find_istio_release_commit() {
  local ver="$1"
  if [[ -n "$FORCE_COMMIT" ]]; then
    printf '%s' "$FORCE_COMMIT"
    return
  fi
  local sha merge
  # Newest commit that touched the RH helm path.
  sha="$(gh api "repos/${ISTIO_RELEASE_REPO}/commits?path=${ver}-redhat/helm&per_page=1" --jq '.[0].sha' 2>/dev/null || true)"
  [[ -n "$sha" && "$sha" != "null" ]] || return 1
  # Prefer the merge commit of the PR that landed that change (matches human PRs).
  merge="$(gh api "repos/${ISTIO_RELEASE_REPO}/commits/${sha}/pulls" --jq '.[0].merge_commit_sha // empty' 2>/dev/null || true)"
  if [[ -n "$merge" && "$merge" != "null" ]]; then
    printf '%s' "$merge"
  else
    printf '%s' "$sha"
  fi
}

rh_chart_urls_exist() {
  local ver="$1" commit="$2" chart url quiet="${3:-}"
  for chart in "${CHARTS[@]}"; do
    url="https://github.com/${ISTIO_RELEASE_REPO}/raw/${commit}/${ver}-redhat/helm/${chart}-${ver}.tgz"
    if ! curl -sfI "$url" >/dev/null; then
      [[ "$quiet" == "quiet" ]] || err "Missing RH chart: $url"
      return 1
    fi
  done
}

list_active_release_branches() {
  if [[ -n "$DISCOVER_BRANCHES" ]]; then
    local b
    IFS=',' read -r -a _discover_branches <<< "$DISCOVER_BRANCHES"
    for b in "${_discover_branches[@]}"; do
      b="$(echo "$b" | tr -d '[:space:]')"
      [[ -n "$b" ]] || continue
      [[ "$b" =~ ^release-[0-9]+\.[0-9]+$ ]] || die "Invalid --branches entry: $b"
      printf '%s\n' "$b"
    done
    return
  fi
  # Only release-X.Y lines (not release-X.Y.Z tags/branches or suffixes).
  gh api "repos/${REPO}/branches?per_page=100" --paginate --jq '.[].name' \
    | grep -E '^release-[0-9]+\.[0-9]+$' \
    | sort -t. -k1,1 -k2,2n \
    | tail -n "${DISCOVER_ACTIVE_COUNT}"
}

# Prints: number<TAB>title  (open PR covering ver on branch), or returns 1.
open_pr_number_for_version_on_branch() {
  local ver="$1" branch="$2"
  local number state base title
  load_previous_rh_prs
  while IFS=$'\t' read -r number state base title; do
    [[ "$state" == "open" ]] || continue
    [[ "$base" == "$branch" ]] || continue
    if grep -Eq "$ver" <<<"$title"; then
      printf '%s\t%s' "$number" "$title"
      return 0
    fi
  done <"$RH_PR_CACHE_FILE"
  return 1
}

open_pr_for_version_on_branch() {
  local ver="$1" branch="$2" row number title
  row="$(open_pr_number_for_version_on_branch "$ver" "$branch" || true)"
  [[ -n "$row" ]] || return 1
  IFS=$'\t' read -r number title <<<"$row"
  printf '#%s %s' "$number" "$title"
}

# Cache ossm/values.yaml patch text for a PR under TMPDIR.
pr_values_patch() {
  local number="$1"
  local cache="${TMPDIR_ROOT}/pr-${number}-values.patch"
  if [[ ! -f "$cache" ]]; then
    gh api "repos/${REPO}/pulls/${number}/files" --paginate \
      --jq '.[] | select(.filename=="ossm/values.yaml") | .patch // empty' \
      >"$cache" 2>/dev/null || true
  fi
  cat "$cache"
}

# Cache full ossm/values.yaml from an open PR head.
pr_head_values_file() {
  local number="$1"
  local cache="${TMPDIR_ROOT}/pr-${number}-values.yaml"
  if [[ ! -f "$cache" ]]; then
    local head_repo head_ref
    head_repo="$(gh api "repos/${REPO}/pulls/${number}" --jq .head.repo.full_name)"
    head_ref="$(gh api "repos/${REPO}/pulls/${number}" --jq .head.ref)"
    gh api "repos/${head_repo}/contents/ossm/values.yaml?ref=${head_ref}" --jq .content \
      | base64 -d >"$cache"
  fi
  printf '%s' "$cache"
}

run_discover() {
  local branches=() ref row branch ver commit first versions_file
  local -a ready_rows=() waiting_rows=()
  local -A branch_versions=()

  info "Discovering RH chart candidates on ${REPO} (newest ${DISCOVER_ACTIVE_COUNT} release-X.Y branches)"
  mapfile -t branches < <(list_active_release_branches)
  ((${#branches[@]})) || die "No release-X.Y branches found on ${REPO}"
  info "Active branches: ${branches[*]}"
  summarize_previous_prs

  for ref in "${branches[@]}"; do
    info "Scanning ${ref}"
    versions_file="${TMPDIR_ROOT}/discover-${ref}.yaml"
    if ! fetch_file "$VERSIONS_YAML_FILE" "$versions_file" "$ref" 2>/dev/null; then
      warn "Skipping ${ref}: no ${VERSIONS_YAML_FILE}"
      continue
    fi
    while IFS= read -r ver; do
      [[ -n "$ver" ]] || continue
      first="$(yq -r ".versions[] | select(.name == \"v${ver}\") | .charts[0]" "$versions_file")"
      if is_rh_chart_url "$first"; then
        continue
      fi
      if ! is_upstream_chart_url "$first"; then
        warn "${ref} v${ver}: unexpected chart URL; skipping"
        continue
      fi
      if commit="$(find_istio_release_commit "$ver")" && rh_chart_urls_exist "$ver" "$commit" quiet; then
        ready_rows+=("${ref}"$'\t'"${ver}"$'\t'"${commit}")
      else
        waiting_rows+=("${ref}"$'\t'"${ver}")
      fi
    done < <(yq -r '.versions[] | select(.ref == null) | select(.eol != true) | .version' "$versions_file")
  done

  printf '\n' >&2
  info "Ready for RH chart swap (upstream charts + RH artifacts published)"
  if ((${#ready_rows[@]})); then
    printf '  %-14s %-10s %-12s %s\n' "BRANCH" "VERSION" "COMMIT" "OPEN_PR" >&2
    for row in "${ready_rows[@]}"; do
      IFS=$'\t' read -r branch ver commit <<<"$row"
      local open_pr=""
      open_pr="$(open_pr_for_version_on_branch "$ver" "$branch" || true)"
      printf '  %-14s %-10s %-12s %s\n' "$branch" "$ver" "${commit:0:12}" "${open_pr:-none}" >&2
    done
  else
    ok "Nothing ready — no upstream-chart versions with published RH charts"
  fi

  if ((${#waiting_rows[@]})); then
    printf '\n' >&2
    info "Still on upstream charts; RH artifacts not in istio-release yet"
    printf '  %-14s %s\n' "BRANCH" "VERSION" >&2
    for row in "${waiting_rows[@]}"; do
      IFS=$'\t' read -r branch ver <<<"$row"
      printf '  %-14s %s\n' "$branch" "$ver" >&2
    done
  fi

  # Machine-readable ready rows on stdout for scripting.
  for row in "${ready_rows[@]}"; do
    printf '%s\n' "$row"
  done

  if [[ "$MODE" == "discover-table" ]]; then
    ok "discover complete — pass --discover --dry-run to plan each ready branch, or re-run with --branch/--versions"
    return 0
  fi

  if ! ((${#ready_rows[@]})); then
    ok "discover ${MODE}: nothing to plan"
    return 0
  fi

  local -a skipped_open=()
  for row in "${ready_rows[@]}"; do
    IFS=$'\t' read -r branch ver commit <<<"$row"
    if ! $INCLUDE_OPEN && open_pr_for_version_on_branch "$ver" "$branch" >/dev/null; then
      skipped_open+=("${branch} ${ver} ($(open_pr_for_version_on_branch "$ver" "$branch"))")
      continue
    fi
    if [[ -n "${branch_versions[$branch]:-}" ]]; then
      branch_versions[$branch]="${branch_versions[$branch]},${ver}"
    else
      branch_versions[$branch]="$ver"
    fi
  done

  if ((${#skipped_open[@]})); then
    printf '\n' >&2
    info "Skipping in-flight versions (open RH PR already exists). Pass --include-open to plan them:"
    local s
    for s in "${skipped_open[@]}"; do
      printf '  - %s\n' "$s" >&2
    done
  fi

  if ! ((${#branch_versions[@]})); then
    ok "discover ${MODE}: nothing left to plan after skipping open PRs"
    return 0
  fi

  # Process highest minor branch first for stable output.
  mapfile -t branches < <(printf '%s\n' "${!branch_versions[@]}" | sort -t. -k2,2nr)
  for branch in "${branches[@]}"; do
    printf '\n' >&2
    info "===== ${MODE} for ${branch}: ${branch_versions[$branch]} ====="
    BRANCH="$branch"
    VERSIONS_CSV="${branch_versions[$branch]}"
    IFS=',' read -r -a VERSIONS <<< "$VERSIONS_CSV"
    if $CREATE_PR && [[ "$MODE" == "dry-run" ]]; then
      info "Would open a human-review PR for ${BRANCH} versions=${VERSIONS_CSV} (dry-run)"
      load_branch_files "$BRANCH"
      run_pipeline
      continue
    fi
    if $CREATE_PR; then
      create_pr_for_branch
    else
      load_branch_files "$BRANCH"
      run_pipeline
    fi
  done
}

work_branch_name() {
  # e.g. add-1.30.8-1.31.4-rh-helm-charts-3.5
  local ossm="${BRANCH#release-}"
  local vers="${VERSIONS_CSV//,/-}"
  printf 'add-%s-rh-helm-charts-%s' "$vers" "$ossm"
}

pr_title_for_versions() {
  local parts=() v
  IFS=',' read -r -a parts <<< "$VERSIONS_CSV"
  if ((${#parts[@]} == 1)); then
    printf 'Add RH %s charts and placeholders' "${parts[0]}"
  else
    local joined=""
    for v in "${parts[@]}"; do
      if [[ -z "$joined" ]]; then
        joined="$v"
      else
        joined+=", ${v}"
      fi
    done
    # Oxford-ish: "1.30.8 and 1.31.4" when two
    if ((${#parts[@]} == 2)); then
      printf 'Add RH %s and %s charts and placeholders' "${parts[0]}" "${parts[1]}"
    else
      printf 'Add RH %s charts and placeholders' "$joined"
    fi
  fi
}

pr_body_for_versions() {
  local parts=() v bullets=""
  IFS=',' read -r -a parts <<< "$VERSIONS_CSV"
  for v in "${parts[@]}"; do
    bullets+="- Point \`v${v}\` chart URLs in \`versions.ossm.yaml\` at istio-release RH charts"$'\n'
  done
  cat <<EOF
Add RH helm charts and Konflux placeholders

${bullets}- Add Konflux image placeholders in \`ossm/values.yaml\`
- Run \`make gen\` to refresh charts and bundle manifests

Please review carefully before merging. This PR was opened by automation and is **not** set to auto-merge.
EOF
}

checkout_release_branch_for_pr() {
  local work
  work="$(work_branch_name)"
  git rev-parse --is-inside-work-tree >/dev/null 2>&1 || die "--create-pr requires a git checkout of ${REPO}"
  [[ -f "$VERSIONS_YAML_FILE" && -f "$HELM_VALUES_FILE" ]] || die "--create-pr requires local ${VERSIONS_YAML_FILE} and ${HELM_VALUES_FILE} (OSSM checkout)"

  info "Fetching ${GIT_REMOTE}/${BRANCH}"
  git fetch "${GIT_REMOTE}" "${BRANCH}"
  # Detach from any dirty state on unrelated branches by creating work branch from remote tip.
  if ! git diff --quiet || ! git diff --cached --quiet; then
    die "Working tree is dirty; commit/stash before --create-pr"
  fi
  git checkout -B "${work}" "${GIT_REMOTE}/${BRANCH}"
  FETCH_REMOTE_FILES=false
  VERSIONS_PATH="$VERSIONS_YAML_FILE"
  VALUES_PATH="$HELM_VALUES_FILE"
  MUST_GATHER_CACHE=()
}

commit_push_open_pr() {
  local work title body label_args=()
  work="$(work_branch_name)"
  title="$(pr_title_for_versions)"
  body="$(pr_body_for_versions)"

  git add \
    "${VERSIONS_YAML_FILE}" \
    "${HELM_VALUES_FILE}" \
    bundle/manifests/servicemeshoperator3.clusterserviceversion.yaml \
    pkg/install/images.gen.go \
    resources/ || true

  # Drop accidental upstream versions.yaml churn if present.
  if git show "HEAD:pkg/istioversion/versions.yaml" >/dev/null 2>&1; then
    git checkout -- pkg/istioversion/versions.yaml 2>/dev/null || true
  fi

  if git diff --cached --quiet; then
    warn "No staged changes for ${BRANCH} / ${VERSIONS_CSV}; skipping PR"
    return 0
  fi

  git -c user.name="${GIT_AUTHOR_NAME:-sail-operator-bot}" \
      -c user.email="${GIT_AUTHOR_EMAIL:-sail-operator-bot@users.noreply.github.com}" \
      commit -s -m "$(printf '%s\n\n%s\n' "$title" "Automated RH chart swap for human review.")"

  info "Pushing ${work} to ${GIT_REMOTE}"
  git push -u "${GIT_REMOTE}" "HEAD:refs/heads/${work}"

  if [[ -n "$PR_LABELS" ]]; then
    local lbl
    IFS=',' read -r -a _lbls <<< "$PR_LABELS"
    for lbl in "${_lbls[@]}"; do
      lbl="$(echo "$lbl" | tr -d '[:space:]')"
      [[ -n "$lbl" ]] || continue
      label_args+=(--label "$lbl")
    done
  fi

  info "Opening PR against ${BRANCH} (human review required — no auto-merge)"
  gh pr create \
    --repo "${REPO}" \
    --base "${BRANCH}" \
    --head "${work}" \
    --title "${title}" \
    --body "${body}" \
    "${label_args[@]+"${label_args[@]}"}"
}

create_pr_for_branch() {
  local start_ref
  start_ref="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
  checkout_release_branch_for_pr
  MODE=apply
  MAKE_GEN=true
  run_pipeline
  commit_push_open_pr
  if [[ -n "$start_ref" && "$start_ref" != "HEAD" ]]; then
    git checkout "${start_ref}" >/dev/null 2>&1 || true
  fi
}

# Returns components present for the newest existing images.v1_MINOR_* block.
# Prefer must-gather when any patch in the minor series has it.
components_for_minor() {
  local minor="$1"
  local key
  key="$(underscore "$minor")"
  python3 - "$VALUES_PATH" "$key" <<'PY'
import re, sys
path, key = sys.argv[1], sys.argv[2]
text = open(path).read()
comps = set()
for m in re.finditer(rf"images\.v{key}_\d+\.([A-Za-z0-9-]+):", text):
    comps.add(m.group(1))
# Stable order used by previous PRs
order = ["cni", "istiod", "must-gather", "proxy", "ztunnel"]
out = [c for c in order if c in comps]
if not out:
    # Brand-new minor: default to full set including must-gather
    out = order
print(",".join(out))
PY
}

# grep -E treats \{ as a quantifier — always match ${VAR} via [$]{VAR}.
placeholder_re() {
  # usage: placeholder_re OSSM_MUST_GATHER_3_4  OR  placeholder_re 'OSSM_MUST_GATHER_[0-9]+_[0-9]+'
  printf '[$]{%s}' "$1"
}

extract_must_gather_from_text() {
  local key="$1" text="$2"
  # Prefer newly added placeholder lines from patches (+…), then any match.
  local hit re
  re="images\\.v${key}_[0-9]+\\.must-gather:[[:space:]]*$(placeholder_re 'OSSM_MUST_GATHER_[0-9]+_[0-9]+')"
  hit="$(grep -Eo "^\+[[:space:]]*${re}" <<<"$text" | head -1 || true)"
  if [[ -z "$hit" ]]; then
    hit="$(grep -Eo "${re}" <<<"$text" | head -1 || true)"
  fi
  [[ -n "$hit" ]] || return 1
  sed -E 's/.*[$]\{(OSSM_MUST_GATHER_[0-9]+_[0-9]+)\}.*/\1/' <<<"$hit"
}

must_gather_var_for_minor() {
  local minor="$1"
  local key existing pr_var head_file
  if [[ -n "${MUST_GATHER_CACHE[$minor]:-}" ]]; then
    printf '%s' "${MUST_GATHER_CACHE[$minor]}"
    return
  fi
  key="$(underscore "$minor")"

  # 1) Current branch values (existing placeholders for this Istio minor).
  existing="$(extract_must_gather_from_text "$key" "$(cat "$VALUES_PATH")" || true)"
  if [[ -n "$existing" ]]; then
    MUST_GATHER_CACHE[$minor]="$existing"
    printf '%s' "$existing"
    return
  fi

  # 2) Open PR head for any requested version in this minor on this branch (authoritative).
  local ver number_title number
  for ver in "${VERSIONS[@]:-}"; do
    [[ "$(istio_minor "$ver")" == "$minor" ]] || continue
    number_title="$(open_pr_number_for_version_on_branch "$ver" "$BRANCH" || true)"
    [[ -n "$number_title" ]] || continue
    IFS=$'\t' read -r number _ <<<"$number_title"
    if head_file="$(pr_head_values_file "$number" 2>/dev/null)"; then
      existing="$(extract_must_gather_from_text "$key" "$(cat "$head_file")" || true)"
      if [[ -n "$existing" ]]; then
        ok "must-gather for ${minor}.x taken from open PR #${number} head: \${${existing}}"
        MUST_GATHER_CACHE[$minor]="$existing"
        printf '%s' "$existing"
        return
      fi
    fi
  done

  # 3) Previous RH PR patches: same base first, then any base (Istio minor ownership
  #    often stays on an older OSSM line, e.g. 1.30.x → OSSM_MUST_GATHER_3_4 on release-3.5).
  pr_var="$(previous_pr_must_gather_var "$minor" same-base || true)"
  if [[ -z "$pr_var" ]]; then
    pr_var="$(previous_pr_must_gather_var "$minor" any-base || true)"
  fi
  if [[ -n "$pr_var" ]]; then
    MUST_GATHER_CACHE[$minor]="$pr_var"
    printf '%s' "$pr_var"
    return
  fi

  # 4) Last resort: current branch OSSM minor — warn because this is often wrong
  #    when cherry-picking an Istio stream onto a newer release line.
  warn "No prior must-gather placeholder found for ${minor}.x; falling back to branch ${BRANCH} → OSSM_MUST_GATHER_$(ossm_minor_from_branch). Verify before apply."
  MUST_GATHER_CACHE[$minor]="OSSM_MUST_GATHER_$(ossm_minor_from_branch)"
  printf '%s' "${MUST_GATHER_CACHE[$minor]}"
}

# Cached rows: number<TAB>state<TAB>base<TAB>title
load_previous_rh_prs() {
  if [[ -s "$RH_PR_CACHE_FILE" ]]; then
    return
  fi
  info "Loading recent RH charts PRs for pattern checks"
  local number state title base
  while IFS=$'\t' read -r number state title; do
    [[ -n "$number" ]] || continue
    base="$(gh api "repos/${REPO}/pulls/${number}" --jq .base.ref)"
    printf '%s\t%s\t%s\t%s\n' "$number" "$state" "$base" "$title" >>"$RH_PR_CACHE_FILE"
  done < <(
    # Broad query: titles vary ("RH charts", "rh helm", "placeholders", …).
    gh api "search/issues?q=repo:${REPO}+is:pr+(RH+OR+rh)+(charts+OR+helm+OR+placeholders)&per_page=30" \
      --jq '.items[] | "\(.number)\t\(.state)\t\(.title)"'
  )
}

previous_pr_must_gather_var() {
  local minor="$1" scope="${2:-same-base}" # same-base | any-base
  local key number state base title patch hit
  key="$(underscore "$minor")"
  load_previous_rh_prs
  while IFS=$'\t' read -r number state base title; do
    if [[ "$scope" == "same-base" && "$base" != "$BRANCH" ]]; then
      continue
    fi
    patch="$(pr_values_patch "$number")"
    [[ -n "$patch" ]] || continue
    hit="$(extract_must_gather_from_text "$key" "$patch" || true)"
    if [[ -n "$hit" ]]; then
      printf '%s' "$hit"
      return 0
    fi
  done <"$RH_PR_CACHE_FILE"
  return 1
}

check_open_prs() {
  local ver conflicts=()
  local number state base title
  load_previous_rh_prs
  while IFS=$'\t' read -r number state base title; do
    [[ "$state" == "open" ]] || continue
    [[ "$base" == "$BRANCH" ]] || continue
    for ver in "${VERSIONS[@]}"; do
      if grep -Eq "$ver" <<<"$title"; then
        conflicts+=("#${number} ${title}")
      fi
    done
  done <"$RH_PR_CACHE_FILE"
  if ((${#conflicts[@]})) && ! $SKIP_PR_CHECK; then
    err "Open RH charts PR(s) already cover requested version(s) on ${BRANCH}:"
    for c in "${conflicts[@]}"; do err "  - $c"; done
    die "Re-run with --skip-pr-check to continue anyway"
  elif ((${#conflicts[@]})); then
    warn "Open RH charts PR(s) already cover requested version(s) (ignored via --skip-pr-check):"
    for c in "${conflicts[@]}"; do warn "  - $c"; done
  else
    ok "No open conflicting RH charts PRs on ${BRANCH}"
  fi
}

summarize_previous_prs() {
  info "Recent RH charts PRs (used as pattern reference)"
  local number state base title shown=0
  load_previous_rh_prs
  while IFS=$'\t' read -r number state base title; do
    printf '  #%s [%s] base=%s  %s\n' "$number" "$state" "$base" "$title" >&2
    shown=$((shown + 1))
    (( shown >= 8 )) && break
  done <"$RH_PR_CACHE_FILE"
}

warn_version_risks() {
  local ver="$1"
  local key minor older
  key="$(underscore "$ver")"
  minor="$(istio_minor "$ver")"

  # Existing digests for this exact version → digest→placeholder refresh (not automated).
  if grep -Eq "images\.v${key}\.[A-Za-z0-9-]+:[[:space:]]*registry\.redhat\.io/" "$VALUES_PATH"; then
    warn "${ver}: ossm/values.yaml already has registry digests for images.v${key}.* — converting digests→placeholders is NOT automated. Edit manually if needed."
  fi

  # Older placeholder-only keys in this minor (rename pattern like v1_31_0 → v1_31_1).
  older="$(grep -Eo "images\.v$(underscore "$minor")_[0-9]+\." "$VALUES_PATH" \
    | sed -E "s/.*v$(underscore "$minor")_([0-9]+)\./\1/" \
    | sort -n | uniq || true)"
  if [[ -n "$older" ]] && ! grep -q "images\.v${key}\." "$VALUES_PATH"; then
    if grep -Eq "images\.v$(underscore "$minor")_[0-9]+\.[A-Za-z0-9-]+:[[:space:]]*[$]{ISTIO_" "$VALUES_PATH"; then
      warn "${ver}: ${minor}.x still has \${…} placeholders on an older patch key — confirm whether this should be a rename (see #1201) instead of adding a new block."
    fi
  fi
}

cross_check_open_pr_placeholders() {
  local ver="$1" comps_csv="$2"
  local row number title head_file key minor planned found
  row="$(open_pr_number_for_version_on_branch "$ver" "$BRANCH" || true)"
  [[ -n "$row" ]] || return 0
  IFS=$'\t' read -r number title <<<"$row"
  head_file="$(pr_head_values_file "$number" 2>/dev/null || true)"
  [[ -n "$head_file" && -f "$head_file" ]] || return 0

  key="$(underscore "$ver")"
  minor="$(istio_minor "$ver")"
  info "Cross-checking planned placeholders for ${ver} against open PR #${number}"

  if ! grep -q "images\.v${key}\." "$head_file"; then
    warn "Open PR #${number} does not yet contain images.v${key}.* — plan may be ahead of that PR tip"
    return 0
  fi

  local c
  IFS=',' read -r -a comps <<< "$comps_csv"
  for c in "${comps[@]}"; do
    case "$c" in
      must-gather)
        planned="$(must_gather_var_for_minor "$minor")"
        found="$(grep -E "images\.v${key}\.must-gather:[[:space:]]*$(placeholder_re 'OSSM_MUST_GATHER_[0-9]+_[0-9]+')" "$head_file" \
          | head -1 \
          | sed -E 's/.*[$]\{(OSSM_MUST_GATHER_[0-9]+_[0-9]+)\}.*/\1/' || true)"
        if [[ -n "$found" && "$found" != "$planned" ]]; then
          err "must-gather mismatch for ${ver}: plan=\${${planned}} open PR #${number}=\${${found}}"
          die "Fix must-gather inference before apply"
        fi
        if [[ -n "$found" ]]; then
          ok "must-gather matches open PR #${number}: \${${found}}"
        fi
        ;;
      cni|istiod|proxy|ztunnel)
        local var
        case "$c" in
          cni) var="ISTIO_CNI_$(underscore "$minor")" ;;
          istiod) var="ISTIO_PILOT_$(underscore "$minor")" ;;
          proxy) var="ISTIO_PROXY_$(underscore "$minor")" ;;
          ztunnel) var="ISTIO_ZTUNNEL_$(underscore "$minor")" ;;
        esac
        if ! grep -Eq "images\.v${key}\.${c}:[[:space:]]*$(placeholder_re "$var")" "$head_file"; then
          warn "Open PR #${number} images.v${key}.${c} does not use \${${var}} — inspect manually"
        fi
        ;;
    esac
  done
}

verify_version() {
  local ver="$1"
  local minor commit status urls first
  minor="$(istio_minor "$ver")"
  info "Verifying ${ver}"

  version_entry_exists "$ver" || die "v${ver} not found in ${VERSIONS_YAML_FILE} on ${BRANCH}. Add upstream charts first."
  ok "versions entry v${ver} exists"

  mapfile -t urls < <(chart_urls_for_version "$ver")
  ((${#urls[@]})) || die "v${ver} has no charts[] entries"
  first="${urls[0]}"
  if is_rh_chart_url "$first"; then
    warn "v${ver} charts already point at RH istio-release URLs"
    status="already-rh"
  elif is_upstream_chart_url "$first"; then
    ok "v${ver} currently uses upstream chart URLs (ready to swap)"
    status="upstream"
  else
    die "v${ver} has unexpected chart URL pattern: $first"
  fi

  commit="$(find_istio_release_commit "$ver")" || die "No ${ver}-redhat/helm path found in ${ISTIO_RELEASE_REPO}"
  ok "istio-release commit for ${ver}: ${commit}"
  rh_chart_urls_exist "$ver" "$commit" || die "RH chart artifacts missing for ${ver} @ ${commit}"
  ok "All 5 RH chart tarballs exist for ${ver}"

  local comps mg
  comps="$(components_for_minor "$minor")"
  ok "Placeholder components for ${minor}.x: ${comps}"
  if [[ ",${comps}," == *",must-gather,"* ]]; then
    mg="$(must_gather_var_for_minor "$minor")"
    ok "must-gather placeholder var: \${${mg}}"
  else
    ok "must-gather omitted for ${minor}.x (matches existing values pattern)"
  fi

  warn_version_risks "$ver"
  cross_check_open_pr_placeholders "$ver" "$comps"

  printf '%s\t%s\t%s\t%s\n' "$ver" "$status" "$commit" "$comps"
}

plan_versions_yaml_edit() {
  local ver="$1" commit="$2"
  cat >&2 <<EOF
  versions.ossm.yaml: rewrite v${ver} charts[] ->
    https://github.com/${ISTIO_RELEASE_REPO}/raw/${commit}/${ver}-redhat/helm/{base,istiod,gateway,cni,ztunnel}-${ver}.tgz
EOF
}

plan_values_edit() {
  local ver="$1" comps_csv="$2"
  local key minor mg
  key="$(underscore "$ver")"
  minor="$(istio_minor "$ver")"
  echo "  ossm/values.yaml: add Konflux placeholders for images.v${key}.*" >&2
  echo "    comment: # ${ver} images will be replaced with Konflux built images and will be updated after next patch release" >&2
  IFS=',' read -r -a comps <<< "$comps_csv"
  local c
  for c in "${comps[@]}"; do
    case "$c" in
      cni) echo "    images.v${key}.cni: \${ISTIO_CNI_$(underscore "$minor")}" >&2 ;;
      istiod) echo "    images.v${key}.istiod: \${ISTIO_PILOT_$(underscore "$minor")}" >&2 ;;
      proxy) echo "    images.v${key}.proxy: \${ISTIO_PROXY_$(underscore "$minor")}" >&2 ;;
      ztunnel) echo "    images.v${key}.ztunnel: \${ISTIO_ZTUNNEL_$(underscore "$minor")}" >&2 ;;
      must-gather)
        mg="$(must_gather_var_for_minor "$minor")"
        echo "    images.v${key}.must-gather: \${${mg}}" >&2
        ;;
    esac
  done
}

apply_versions_yaml() {
  local ver="$1" commit="$2"
  local chart urls=()
  for chart in "${CHARTS[@]}"; do
    urls+=("https://github.com/${ISTIO_RELEASE_REPO}/raw/${commit}/${ver}-redhat/helm/${chart}-${ver}.tgz")
  done
  # Build a YAML list and replace charts for the version entry.
  local yaml_list=""
  local u
  for u in "${urls[@]}"; do
    yaml_list+="\"${u}\","
  done
  yaml_list="[${yaml_list%,}]"
  yq -i "(.versions[] | select(.name == \"v${ver}\") | .charts) = ${yaml_list}" "$VERSIONS_YAML_FILE"
}

apply_values_yaml() {
  local ver="$1" comps_csv="$2"
  local key minor mg block
  key="$(underscore "$ver")"
  minor="$(istio_minor "$ver")"
  if grep -Eq "images\.v${key}\.[A-Za-z0-9-]+:[[:space:]]*registry\.redhat\.io/" "$HELM_VALUES_FILE"; then
    die "${HELM_VALUES_FILE} already has registry digests for images.v${key}.* — digest→placeholder refresh is not automated."
  fi
  if grep -q "images\.v${key}\." "$HELM_VALUES_FILE"; then
    die "${HELM_VALUES_FILE} already contains images.v${key}.*; refusing to duplicate. Edit manually or remove the block first."
  fi

  IFS=',' read -r -a comps <<< "$comps_csv"
  block="    # ${ver} images will be replaced with Konflux built images and will be updated after next patch release"$'\n'
  local c
  for c in "${comps[@]}"; do
    case "$c" in
      cni) block+="    images.v${key}.cni: \${ISTIO_CNI_$(underscore "$minor")}"$'\n' ;;
      istiod) block+="    images.v${key}.istiod: \${ISTIO_PILOT_$(underscore "$minor")}"$'\n' ;;
      proxy) block+="    images.v${key}.proxy: \${ISTIO_PROXY_$(underscore "$minor")}"$'\n' ;;
      ztunnel) block+="    images.v${key}.ztunnel: \${ISTIO_ZTUNNEL_$(underscore "$minor")}"$'\n' ;;
      must-gather)
        mg="$(must_gather_var_for_minor "$minor")"
        block+="    images.v${key}.must-gather: \${${mg}}"$'\n'
        ;;
    esac
  done

  python3 - "$HELM_VALUES_FILE" "$minor" "$block" <<'PY'
import re, sys
path, minor, block = sys.argv[1], sys.argv[2], sys.argv[3]
key = minor.replace(".", "_")
text = open(path).read()
# Insert after the last images.vMINOR_* line (or after the Konflux comment for that minor).
pattern = re.compile(rf"(.*images\.v{key}_\d+\.[A-Za-z0-9-]+:[^\n]*\n)", re.S)
matches = list(re.finditer(rf"images\.v{key}_\d+\.[A-Za-z0-9-]+:[^\n]*\n", text))
if matches:
    last = matches[-1]
    text = text[: last.end()] + block + text[last.end() :]
else:
    # No prior images for this minor: insert before `service:` under deployment annotations.
    m = re.search(r"(?m)^service:\n", text)
    if not m:
        raise SystemExit("Could not find insertion point (service:) in values file")
    text = text[: m.start()] + block + text[m.start() :]
open(path, "w").write(text)
PY
}

run_pipeline() {
  local row ver status commit comps
  local -a VERIFY_ROWS=()

  info "Mode=${MODE}  repo=${REPO}  branch=${BRANCH}  versions=${VERSIONS_CSV}"
  summarize_previous_prs
  check_open_prs

  for ver in "${VERSIONS[@]}"; do
    row="$(verify_version "$ver")"
    VERIFY_ROWS+=("$row")
  done

  info "Planned edits"
  for row in "${VERIFY_ROWS[@]}"; do
    IFS=$'\t' read -r ver status commit comps <<<"$row"
    plan_versions_yaml_edit "$ver" "$commit"
    if [[ "$status" == "already-rh" ]]; then
      warn "Skipping versions.ossm.yaml rewrite for ${ver} (already RH)"
    fi
    if grep -q "images\.v$(underscore "$ver")\." "$VALUES_PATH"; then
      warn "ossm/values.yaml already has images.v$(underscore "$ver").* — placeholders would be skipped"
    else
      plan_values_edit "$ver" "$comps"
    fi
  done

  if [[ "$MODE" == "verify-only" || "$MODE" == "dry-run" ]]; then
    ok "${MODE} complete for ${BRANCH} — no files written"
    return 0
  fi

  info "Applying local edits"
  for row in "${VERIFY_ROWS[@]}"; do
    IFS=$'\t' read -r ver status commit comps <<<"$row"
    if [[ "$status" != "already-rh" ]]; then
      apply_versions_yaml "$ver" "$commit"
      ok "Updated charts for v${ver} in ${VERSIONS_YAML_FILE}"
    fi
    if grep -q "images\.v$(underscore "$ver")\." "$HELM_VALUES_FILE"; then
      warn "Skipping placeholders for ${ver} (already present)"
    else
      apply_values_yaml "$ver" "$comps"
      ok "Added Konflux placeholders for ${ver} in ${HELM_VALUES_FILE}"
    fi
  done

  if $MAKE_GEN; then
    info "Running make gen (OSSM values/versions)"
    # Clear chart/values annotations so only ossm/values.yaml annotations win (merge_upstream convention).
    if [[ -f chart/values.yaml ]]; then
      yq -i '.deployment.annotations = {}' chart/values.yaml
    fi
    VERSIONS_YAML_FILE=versions.ossm.yaml HELM_VALUES_FILE=ossm/values.yaml make gen
    # Midstream PRs usually leave upstream versions.yaml alone.
    if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
      if git show "HEAD:pkg/istioversion/versions.yaml" >/dev/null 2>&1; then
        git checkout -- pkg/istioversion/versions.yaml 2>/dev/null || true
      fi
    fi
    ok "make gen finished"
  else
    cat <<EOF

Applied on ${BRANCH}. Finish with:
  yq -i '.deployment.annotations = {}' chart/values.yaml   # if needed on OSSM
  VERSIONS_YAML_FILE=versions.ossm.yaml HELM_VALUES_FILE=ossm/values.yaml make gen
  # or re-run apply with --make-gen
  git status
EOF
  fi
}

# ---- main ----
if $CREATE_PR && [[ "$MODE" == "apply" ]]; then
  MAKE_GEN=true
fi

if $DISCOVER; then
  # Bare --discover → table. --discover --dry-run → plan. --discover --create-pr → open PRs.
  if [[ "$MODE" == "apply" ]] && ! $CREATE_PR; then
    MODE="discover-table"
  fi
  run_discover
  exit 0
fi

[[ -n "$BRANCH" ]] || die "--branch is required (or use --discover)"
[[ -n "$VERSIONS_CSV" ]] || die "--versions is required (or use --discover)"
[[ "$BRANCH" =~ ^release-[0-9]+\.[0-9]+$ ]] || die "--branch must look like release-3.4 (got: $BRANCH)"

IFS=',' read -r -a VERSIONS <<< "$VERSIONS_CSV"
for v in "${VERSIONS[@]}"; do
  [[ "$v" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "Invalid version '$v' (expected X.Y.Z)"
done

if $CREATE_PR && [[ "$MODE" == "dry-run" ]]; then
  info "Would open a human-review PR for ${BRANCH} versions=${VERSIONS_CSV} (dry-run)"
  load_branch_files "$BRANCH"
  run_pipeline
elif $CREATE_PR; then
  create_pr_for_branch
else
  load_branch_files "$BRANCH"
  run_pipeline
fi

if [[ "$MODE" == "verify-only" || "$MODE" == "dry-run" ]]; then
  cat <<EOF

Next (apply / PR):
  1. Check out ${REPO}@${BRANCH} with a clean worktree
  2. ./hack/add-rh-istio-charts.sh --branch ${BRANCH} --versions ${VERSIONS_CSV} --create-pr
     (or --make-gen then commit/PR manually)
  Human review required — do not auto-merge.
EOF
fi
