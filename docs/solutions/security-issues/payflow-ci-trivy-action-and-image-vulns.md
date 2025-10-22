---
title: PayFlow CI — Aquasecurity Trivy action tag resolution and image HIGH/CRITICAL scan failures
category: security-issues
module: payflow-app
problem_type: security_issue
component: payments
symptoms:
  - "GitHub Actions: Unable to resolve action `aquasecurity/trivy-action@0.28.0`, unable to find version `0.28.0`."
  - "After updating the action ref, `aquasecurity/trivy-action` ran successfully but exited with code 1: Trivy reported 11 vulnerabilities on the `api` gobinary (HIGH: 9, CRITICAL: 2), including `golang.org/x/crypto` CVEs and Go stdlib CVEs for embedded version `v1.22.12`."
root_cause: incomplete_setup
resolution_type: dependency_update
severity: high
tags:
  - trivy
  - github-actions
  - docker
  - golang
  - aquasecurity
  - supply-chain
  - payflow-app
---

# PayFlow CI — Aquasecurity Trivy action tag resolution and image HIGH/CRITICAL scan failures

## Problem

The **`payflow-app`** workflow’s **docker build + trivy** job failed twice in sequence: first the workflow could not resolve a **pinned `aquasecurity/trivy-action` ref**, then—with a valid ref—**Trivy’s default `exit-code: 1` on CRITICAL,HIGH** caused the job to fail because the built API image contained **outdated Go stdlib** and **`golang.org/x/crypto`** as seen by the gobinary scanner.

## Symptoms

- **Action resolution:** `Unable to resolve action aquasecurity/trivy-action@0.28.0`, `unable to find version 0.28.0` during workflow setup (upstream removed or never published that ref under the post–supply-chain tag policy).
- **Scan failure:** Log showed `Total: 11 (HIGH: 9, CRITICAL: 2)` on target `api` (gobinary), with `stdlib` tied to **Go 1.22.12** and **`golang.org/x/crypto` v0.28.0** in the table output; process exited **1** because the job passes **`exit-code: "1"`** when findings match configured severities.

## What Didn't Work

- **Keeping only the old action pin** (`@0.28.0`) — GitHub cannot clone a non-existent tag; the fix required **changing the workflow ref** to a tag that exists on `aquasecurity/trivy-action` (maintainers document **`v`-prefixed** tags, e.g. `v0.35.0`).
- **Assuming “green” CI after the action fix alone** — Trivy was behaving as configured; the remaining failure was **real findings** in the image, not a broken scanner. Lowering severity gates without upgrading the image would only hide the issue.

## Solution

1. **Workflow — pin a published Trivy action version**

   In `.github/workflows/payflow-app.yml`, replace the dead ref with a current tag (example used in repo):

   ```yaml
   - uses: aquasecurity/trivy-action@v0.35.0
   ```

   Upstream release notes for `aquasecurity/trivy-action` describe migration to **`v`-prefixed** tags; older bare numeric tags may be absent.

2. **Image — align builder Go and module deps with patched versions**

   - **`payflow-app/Dockerfile`:** use a **current patch** of Go on Debian for the build stage, e.g. `FROM golang:1.26.2-bookworm AS build` (previously `golang:1.22-bookworm`, which embedded stdlib **1.22.x** in the binary Trivy flagged).
   - **`payflow-app/go.mod`:** set **`go 1.26`** (or match the builder’s minor line) and bump **`golang.org/x/crypto`** to a release at or above the **fixed versions** cited in the Trivy report (e.g. **v0.40.0** after `go get` / `go mod tidy`).
   - Run **`go mod tidy`**, **`go mod verify`**, **`go test ./...`**, then rebuild the API image and re-scan.

3. **Local verification (optional)**

   ```bash
   docker build -f payflow-app/Dockerfile --target api -t payflow-api:ci payflow-app
   docker run --rm -v /var/run/docker.sock:/var/run/docker.sock aquasec/trivy:0.69.3 image \
     --severity CRITICAL,HIGH --exit-code 0 payflow-api:ci
   ```

   Adjust Trivy image tag to stay near the version bundled by `trivy-action` if you want parity with CI.

## Why This Works

- **Action ref:** GitHub Actions resolves actions by **git ref** on the action repository; if the tag was removed or never existed under that exact string, setup fails before any scan runs.
- **Scanner exit 1:** With **`exit-code: "1"`** and **`severity: CRITICAL,HIGH`**, Trivy fails the job when the vulnerability DB reports matching issues on the **filesystem / language artifacts** in the image. Upgrading the **compiler/stdlib** (via the Go builder image) and **direct modules** (e.g. `golang.org/x/crypto`) removes or downgrades those findings to levels outside the gate (or clears them), so the step completes without weakening policy.

## Prevention

- **Pin Trivy action by an existing `v`-prefixed tag** (or full commit SHA) and **re-check** when upgrading the action major/minor line; watch `aquasecurity/trivy-action` release notes for tag policy changes.
- **Keep `Dockerfile` builder Go version and `go.mod` `go` directive in the same supported line** as local dev and CI **`setup-go`** (`go-version-file`), so the compiled stdlib matches expectations and security backports.
- **After dependency or Go version bumps**, run **`docker build` + Trivy** locally (or in a branch workflow) before merging if **`exit-code: 1`** is enforced on HIGH/CRITICAL.
- **Periodic `go list -m -u all` / Dependabot** for `golang.org/x/*` and stdlib-related advisories; Trivy’s DB updates frequently—what passes today can fail on a later DB without code changes.

## Related Issues

- Workflow: `.github/workflows/payflow-app.yml` (fmt/vet/test, Docker `api`/`worker` targets, Trivy step).
- App build: `payflow-app/Dockerfile`, `payflow-app/go.mod`, `payflow-app/go.sum`.
- CI layout (Pattern A): `docs/contracts/release-checklist.md` (links back to this learning).
- Async/worker behavior (separate topic): `docs/solutions/best-practices/payflow-async-plane-worker-webhooks.md`.
