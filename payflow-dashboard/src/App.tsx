import { useCallback, useEffect, useState } from "react";

type Me = {
  tenant_id: string;
  name: string;
  principal: string;
};

export function App() {
  const [apiKey, setApiKey] = useState("");
  const [tenantId, setTenantId] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [token, setToken] = useState(() => sessionStorage.getItem("payflow_token") ?? "");
  const [error, setError] = useState<string | null>(null);
  const [me, setMe] = useState<Me | null>(null);
  const [keysJson, setKeysJson] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const persistToken = useCallback((t: string) => {
    setToken(t);
    if (t) {
      sessionStorage.setItem("payflow_token", t);
    } else {
      sessionStorage.removeItem("payflow_token");
    }
  }, []);

  useEffect(() => {
    if (!token) {
      return;
    }
    let cancelled = false;
    void (async () => {
      const res = await fetch("/v1/tenants/me", {
        headers: { Authorization: `Bearer ${token}` },
      });
      const raw = await res.text();
      if (cancelled) {
        return;
      }
      if (!res.ok) {
        persistToken("");
        setError(`${res.status}: ${raw}`);
        return;
      }
      setMe(JSON.parse(raw) as Me);
    })();
    return () => {
      cancelled = true;
    };
  }, [token, persistToken]);

  const resolveTenant = async () => {
    setError(null);
    setBusy(true);
    try {
      const res = await fetch("/v1/tenants/me", {
        headers: { "X-Api-Key": apiKey.trim() },
      });
      const raw = await res.text();
      if (!res.ok) {
        setError(`${res.status}: ${raw}`);
        return;
      }
      const j = JSON.parse(raw) as Me;
      setTenantId(j.tenant_id);
      setMe(j);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  const createDashboardUser = async () => {
    setError(null);
    setBusy(true);
    try {
      const tid = tenantId.trim();
      if (!tid) {
        setError("Resolve tenant with your API key first.");
        return;
      }
      const res = await fetch(`/v1/tenants/${tid}/dashboard-users`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Api-Key": apiKey.trim(),
        },
        body: JSON.stringify({ email: email.trim(), password }),
      });
      const raw = await res.text();
      if (!res.ok) {
        setError(`${res.status}: ${raw}`);
        return;
      }
      setError(null);
      alert("Dashboard user created. You can sign in below.");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  const login = async () => {
    setError(null);
    setBusy(true);
    try {
      const res = await fetch("/v1/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          tenant_id: tenantId.trim(),
          email: email.trim(),
          password,
        }),
      });
      const raw = await res.text();
      if (!res.ok) {
        setError(`${res.status}: ${raw}`);
        return;
      }
      const j = JSON.parse(raw) as { access_token: string };
      persistToken(j.access_token);
      await loadMeBearer(j.access_token);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  const loadMeBearer = async (t: string) => {
    const res = await fetch("/v1/tenants/me", {
      headers: { Authorization: `Bearer ${t}` },
    });
    const raw = await res.text();
    if (!res.ok) {
      setError(`${res.status}: ${raw}`);
      return;
    }
    setMe(JSON.parse(raw) as Me);
  };

  const loadApiKeys = async () => {
    setError(null);
    setKeysJson(null);
    if (!token) {
      setError("Sign in first.");
      return;
    }
    setBusy(true);
    try {
      const res = await fetch("/v1/tenants/me/api-keys", {
        headers: { Authorization: `Bearer ${token}` },
      });
      const raw = await res.text();
      if (!res.ok) {
        setError(`${res.status}: ${raw}`);
        return;
      }
      setKeysJson(JSON.stringify(JSON.parse(raw), null, 2));
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  const logout = () => {
    persistToken("");
    setMe(null);
    setKeysJson(null);
  };

  return (
    <>
      <h1>PayFlow</h1>
      <p className="sub">Lightweight dashboard (R7). Dev server proxies <code>/v1</code> to the API on port 8080.</p>

      <div className="card">
        <label htmlFor="apiKey">Integration API key</label>
        <input
          id="apiKey"
          autoComplete="off"
          value={apiKey}
          onChange={(e) => setApiKey(e.target.value)}
          placeholder="pf_live_…"
        />
        <div className="row">
          <button type="button" className="secondary" disabled={busy} onClick={() => void resolveTenant()}>
            Resolve tenant
          </button>
        </div>
        {error && <p className="err">{error}</p>}
      </div>

      <div className="card">
        <label htmlFor="tenantId">Tenant ID</label>
        <input
          id="tenantId"
          value={tenantId}
          onChange={(e) => setTenantId(e.target.value)}
          placeholder="uuid"
        />
        <label htmlFor="email">Email</label>
        <input id="email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
        <label htmlFor="password">Password (min 10 chars)</label>
        <input
          id="password"
          type="password"
          autoComplete="new-password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
        />
        <div className="row">
          <button type="button" className="secondary" disabled={busy} onClick={() => void createDashboardUser()}>
            Create dashboard user
          </button>
          <button type="button" disabled={busy} onClick={() => void login()}>
            Sign in (JWT)
          </button>
        </div>
      </div>

      {token && (
        <div className="card">
          <div className="row">
            <span>Signed in</span>
            <button type="button" className="secondary" onClick={logout}>
              Sign out
            </button>
            <button type="button" disabled={busy} onClick={() => void loadApiKeys()}>
              Load API keys
            </button>
          </div>
        </div>
      )}

      {me && (
        <div className="card">
          <strong>/v1/tenants/me</strong>
          <pre>{JSON.stringify(me, null, 2)}</pre>
        </div>
      )}

      {keysJson && (
        <div className="card">
          <strong>/v1/tenants/me/api-keys</strong>
          <pre>{keysJson}</pre>
        </div>
      )}
    </>
  );
}
