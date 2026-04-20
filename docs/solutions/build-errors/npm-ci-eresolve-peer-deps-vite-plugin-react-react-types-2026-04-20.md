---
title: "CI fails on npm ci due to npm ERESOLVE peer dependency conflicts (Vite + React)"
date: "2026-04-20"
category: "build-errors"
module: "payflow-dashboard"
problem_type: "build_error"
component: "tooling"
symptoms:
  - "CI job fails during `npm ci` with `npm ERR! code ERESOLVE`"
  - "`@vitejs/plugin-react` peer requirements do not match the installed `vite` major"
  - "`react` and `react-dom` (or `@types/react` and `@types/react-dom`) resolve to different majors (18 vs 19)"
root_cause: "config_error"
resolution_type: "dependency_update"
severity: "high"
tags:
  - "ci"
  - "npm"
  - "npm-ci"
  - "eresolve"
  - "peer-deps"
  - "vite"
  - "react"
  - "dependabot"
---

# CI fails on npm ci due to npm ERESOLVE peer dependency conflicts (Vite + React)

## Problem

The `payflow-dashboard/` CI workflow failed at `npm ci` because the dependency graph included **incompatible peer dependency combinations**, causing npm to exit with `ERESOLVE`.

## Symptoms

- `npm ERR! code ERESOLVE`
- `npm ERR! ERESOLVE could not resolve`
- `@vitejs/plugin-react@4.x` with `vite@8.x` (plugin peer only supports Vite 4–7)
- `react`/`react-dom` major mismatch (18 vs 19) and/or `@types/react`/`@types/react-dom` major mismatch (18 vs 19)

## What Didn't Work

- Allowing semver ranges (for example `^18.x`) without additional constraints: this permitted automated dependency bumps to land on **mixed majors** across packages that must be kept in lockstep.

## Solution

### 1) Align Vite and `@vitejs/plugin-react`

In `payflow-dashboard/package.json`, keep the Vite major and the React plugin major compatible:

- Set `devDependencies.vite` to `^8.0.8`
- Set `devDependencies.@vitejs/plugin-react` to `^6.0.1` (peer `vite: ^8.0.0`)

Regenerate the lockfile and confirm `npm ci` works:

```bash
cd payflow-dashboard
rm -rf node_modules package-lock.json
npm install
rm -rf node_modules
npm ci
```

### 2) Pin React runtime packages together

In `payflow-dashboard/package.json`, pin the runtime dependencies to the same exact version:

- `dependencies.react`: `18.3.1`
- `dependencies.react-dom`: `18.3.1`

This prevents Dependabot or ad-hoc edits from bumping one package to a different major than the other (which triggers peer conflicts).

### 3) Pin React type packages together and enforce via overrides

In `payflow-dashboard/package.json`, pin and override the React type packages:

- `devDependencies.@types/react`: `18.3.28`
- `devDependencies.@types/react-dom`: `18.3.7`
- `overrides`:
  - `@types/react: 18.3.28`
  - `@types/react-dom: 18.3.7`

Then regenerate the lockfile (or run `npm install`) so CI uses the same resolved graph.

### 4) Prevent Dependabot from reintroducing incompatible “single-package” bumps

In `.github/dependabot.yml` (for `/payflow-dashboard`):

- Group `react`, `react-dom`, `@types/react`, and `@types/react-dom` into one update group for **minor/patch** updates.
- Ignore **semver-major** updates for that group (to avoid React 19 bumps until the dashboard is intentionally upgraded end-to-end).

## Why This Works

- `npm ci` uses the lockfile and enforces peer dependency constraints. When peer ranges conflict, npm stops with `ERESOLVE`.
- Vite 8 requires `@vitejs/plugin-react` 6.x to satisfy the plugin’s peer dependency on `vite`.
- React runtime and React types packages are effectively “lockstep” dependencies: mixing majors (18/19) is incompatible under npm’s peer resolution rules.
- `overrides` makes the type-package major consistent even if transitive dependencies request different versions.

## Prevention

- Keep `react` and `react-dom` pinned to the same version in `payflow-dashboard/package.json`.
- Keep `@types/react` and `@types/react-dom` pinned and enforced via `overrides`.
- Keep `vite` and `@vitejs/plugin-react` on compatible major versions.
- Prefer grouped dependency updates for lockstep packages (configured via `.github/dependabot.yml`) instead of separate PRs that bump one package at a time.

## Related Issues

- `docs/solutions/security-issues/payflow-ci-trivy-action-and-image-vulns.md` (CI failure patterns; different ecosystem)

