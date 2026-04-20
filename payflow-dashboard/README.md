# payflow-dashboard

Vite + React + TypeScript SPA for **R7** (dashboard session path). In development, `vite` proxies `/v1`, `/healthz`, and `/metrics` to `http://127.0.0.1:8080` so the browser does not need CORS.

## Run

1. Start Postgres, Redis, `payflow-app` API, and worker per `payflow-app/README.md`.
2. From this directory:

```bash
npm ci
npm run dev
```

Open `http://localhost:5173`. Use an integration API key to resolve the tenant, create a dashboard user (same key), then sign in with JWT to load API keys.

## Production build

```bash
npm ci
npm run build
```

Serve `dist/` behind the same origin as the API, or set `CORS_ALLOWED_ORIGINS` on the API to your dashboard origin.
