# Add RH Istio charts and Konflux placeholders (OSSM)

Automates the midstream pattern used by PRs like
[#1308](https://github.com/openshift-service-mesh/sail-operator/pull/1308),
[#1307](https://github.com/openshift-service-mesh/sail-operator/pull/1307),
[#1164](https://github.com/openshift-service-mesh/sail-operator/pull/1164):
swap upstream helm chart URLs in `versions.ossm.yaml` for Red Hat charts from
`openshift-service-mesh/istio-release`, add Konflux image placeholders in
`ossm/values.yaml`, run `make gen`, and open a PR against an OSSM `release-3.X`
branch.

## Modes

Ask the user which mode to run (default **discover** if they want “all versions”, otherwise **dry-run**):

| Mode | Behavior |
|------|----------|
| `discover` | Scan active OSSM `release-X.Y` branches (default: newest **5**, covering 3.1–3.5 today); list versions still on upstream charts that already have RH artifacts. Override with `DISCOVER_ACTIVE_COUNT`. Works from **any** checkout (including upstream `main`) via GitHub API |
| `discover --dry-run` | Same scan, then plan edits for ready sets **without** an open RH PR (safe default). Use `--include-open` to also plan in-flight versions and cross-check placeholders against those PR heads |
| `dry-run` | Single branch: verify + print planned edits, **no writes / no PR** |
| `verify-only` | Single branch: prerequisite / previous-PR checks only |
| `apply` | Single branch only: apply edits locally, `make gen`, commit, push, open PR |
| `create-pr` | Apply + `make gen` + commit/push + open a **human-review** PR (no auto-merge). Use with `--discover` to open one PR per ready release line |

CI: `.github/workflows/add-rh-istio-charts.yaml` (OSSM repo only) runs discover on `versions.ossm.yaml` pushes to `release-3.*` and on a daily cron, then opens review PRs. `workflow_dispatch` defaults to **dry_run=true**.

Always run `hack/add-rh-istio-charts.sh` for verification / planning before any apply.
`--discover --create-pr` opens one PR per branch; it never auto-merges.

## Prerequisites

- `gh` authenticated (`gh auth status`)
- `yq`, `curl`, `python3`
- For **apply**: clean checkout of `openshift-service-mesh/sail-operator` on (or branched from) the target `release-3.X` branch, with `pkg/istioversion/versions.ossm.yaml` and `ossm/values.yaml` present
- RH charts already published under `openshift-service-mesh/istio-release` as `X.Y.Z-redhat/helm/*.tgz`
- Target version(s) already present in `versions.ossm.yaml` with **upstream** chart URLs (from a prior “add upstream charts” PR). If missing, STOP and tell the user to land upstream charts first

## Inputs to collect

1. **Scope** — all active branches (`discover`) vs one branch
2. If single-branch: **OSSM branch** (e.g. `release-3.4`) and **Istio version(s)**
3. **Mode** — `discover` / `discover --dry-run` / `dry-run` / `verify-only` / `apply`
4. Optional: stack/base branch tip if this must sit on top of an open upstream-charts PR

Prefer **`--discover`** when the user says “all versions”, “everything ready”, or “from main”.

## Tasks

### 1. Detect remotes

- Find the OSSM remote (URL contains `openshift-service-mesh/sail-operator`). If missing, ask to add it or use `gh` + `--fetch-remote` / `--discover` for remote-only runs.
- Find the fork remote for push (usually `origin` under the user’s GitHub user).
- Do **not** assume remote names.
- Discover/dry-run can run from upstream `main`; apply still needs an OSSM checkout.

### 2. Run the helper script (required)

From the repo root (works on upstream main — no local `ossm/` needed):

```bash
# Scan all active OSSM release lines
./hack/add-rh-istio-charts.sh --discover

# Scan + plan ready swaps that are NOT already in an open PR
./hack/add-rh-istio-charts.sh --discover --dry-run

# Also plan / cross-check against in-flight open PRs (e.g. #1307/#1308)
./hack/add-rh-istio-charts.sh --discover --dry-run --include-open

# Open human-review PRs for every ready branch (skips in-flight open PRs)
./hack/add-rh-istio-charts.sh --discover --create-pr

# Single-branch dry-run
./hack/add-rh-istio-charts.sh --dry-run \
  --branch release-3.4 \
  --versions 1.30.8 \
  --fetch-remote
```

The script:

1. Lists recent RH charts PRs on `openshift-service-mesh/sail-operator` as pattern reference
2. Fails if an **open** PR on the same base already covers the requested version(s) (override with `--skip-pr-check`)
3. Confirms each version exists in `versions.ossm.yaml`
4. Confirms charts are still upstream (`blob.istio.io` / `istio-release.storage.googleapis.com`) or reports already-RH
5. Resolves the `istio-release` commit that published `X.Y.Z-redhat/helm` and HEAD-checks all five tarballs
6. Infers placeholder **components** from existing `images.vX_Y_*` entries on that branch (e.g. `1.31.x` often omits `must-gather`; `1.30.x` includes it)
7. Infers `${OSSM_MUST_GATHER_X_Y}` from existing values or prior RH PRs on the same base (falls back to the branch minor)
8. Prints the planned `versions.ossm.yaml` + `ossm/values.yaml` edits

**Show the script output to the user.** On failure, STOP and explain what is missing (upstream charts, istio-release publish, conflicting PR, etc.).

### 3. Dry-run / verify-only stop here

If mode is `dry-run` or `verify-only`:

- Summarize pass/fail per version
- Paste the planned edits
- Ask whether to proceed with **apply**
- Do **not** modify files, commit, or open a PR

### 4. Apply (only when user confirmed)

1. Ensure working tree is clean and based on latest `openshift-service-mesh/sail-operator` `<branch>` (fetch first).
2. Create working branch, e.g. `add-1.30.8-rh-helm-charts-3.4` (include all versions + OSSM minor).
3. Run apply (local files required):

```bash
# Prefer --make-gen so the PR includes charts/CSV like prior RH PRs
./hack/add-rh-istio-charts.sh \
  --branch release-3.4 \
  --versions 1.30.8 \
  --make-gen
```

Without `--make-gen`, regenerate manually (OSSM convention):

```bash
yq -i '.deployment.annotations = {}' chart/values.yaml
VERSIONS_YAML_FILE=versions.ossm.yaml HELM_VALUES_FILE=ossm/values.yaml make gen
```

5. If `make gen` also rewrote `pkg/istioversion/versions.yaml` (upstream file) with unrelated churn, restore it to the branch tip unless the user wants that change — midstream PRs usually touch **`versions.ossm.yaml` only**. `--make-gen` attempts this restore automatically.

6. Review `git status` / `git diff`. Expected touch set mirrors prior PRs:
   - `pkg/istioversion/versions.ossm.yaml`
   - `ossm/values.yaml`
   - `bundle/manifests/servicemeshoperator3.clusterserviceversion.yaml`
   - `pkg/install/images.gen.go` (sometimes)
   - `resources/vX.Y.Z/**` (etags, charts, OpenShift profile tweaks from RH charts)

### 5. Commit, push, PR

- Sign commits with `-s`
- Suggested messages (split if helpful):
  - `Add RH X.Y.Z charts and Konflux placeholders`
- Push to the fork and open PR against `openshift-service-mesh/sail-operator` base `<branch>`:

```bash
gh pr create --repo openshift-service-mesh/sail-operator \
  --base release-3.4 \
  --title "Add RH 1.30.8 charts and placeholders" \
  --body "$(cat <<'EOF'
Add RH 1.30.8 helm charts and Konflux placeholders

- Point `v1.30.8` chart URLs in `versions.ossm.yaml` at istio-release RH charts
- Add Konflux image placeholders in `ossm/values.yaml`
- Run `make gen` to refresh charts and bundle manifests
EOF
)"
```

Adjust title/body for multiple versions. Mention any stacked dependency PR.

### 6. Summary

Return:

- Mode run
- Verification results (per version: upstream→RH commit, components, must-gather var)
- Whether files were written
- PR URL (apply only)
- Any warnings (already-RH, skipped placeholders, conflicting PRs overridden)

## Pattern rules (from previous PRs)

- Chart URL form:
  `https://github.com/openshift-service-mesh/istio-release/raw/<commit>/<ver>-redhat/helm/<chart>-<ver>.tgz`
- Placeholder comment:
  `# <ver> images will be replaced with Konflux built images and will be updated after next patch release`
- Image keys: `images.v1_30_8.cni` etc. (dots → underscores)
- Component vars: `${ISTIO_CNI_1_30}`, `${ISTIO_PILOT_1_30}`, `${ISTIO_PROXY_1_30}`, `${ISTIO_ZTUNNEL_1_30}`
- must-gather (when the minor series uses it): `${OSSM_MUST_GATHER_3_4}` — **not always** the current branch; copy from prior PR / existing placeholder for that Istio minor
- Do not invent digests; placeholders stay until Konflux builds land in a later PR

## Important notes

- Default to **dry-run** / **discover**. Never apply or open a PR unless the user asks.
- Prefer the helper script over hand-editing so verification stays consistent.
- If RH charts are missing in `istio-release`, STOP — do not point at a guessed commit.
- If the version is absent from `versions.ossm.yaml`, STOP — upstream charts PR is a prerequisite.
- Open conflicting PRs are a hard fail unless the user passes `--skip-pr-check` / confirms override.
- `--discover` annotates ready rows with existing open PRs (so you do not recreate #1307/#1308).
- Chart URLs use the **merge commit** of the istio-release PR when available (same as hand-written PRs).
- Active lines: default newest 5 (`3.1`–`3.5`). Override with `--branches release-3.1,release-3.2,...` or `DISCOVER_BRANCHES`.

## Known gaps (handle manually)

- **Placeholder rename** (e.g. `images.v1_31_0.*` → `images.v1_31_1.*` as in #1201): script only **adds** a new version block; it does not rename keys.
- **Digest → placeholder refresh** on an existing version (e.g. #1307 rewriting `1.28.10` digests back to `${…}`): not automated; only new version placeholders are added.
- **Stacked upstream-charts PRs**: discover reads the **merged** `release-X.Y` tip. If upstream charts exist only on an unmerged stack branch, pass `--branch`/`--versions` against that tip (or wait for merge).
- This automation is OSSM-oriented; land the script/command on a dedicated PR to `main` (or the OSSM default branch), not on an unrelated feature branch.
