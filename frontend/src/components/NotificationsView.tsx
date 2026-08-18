"use client";

import { useState } from "react";
import { apiFetch } from "@/lib/auth";
import type { NotifLog, NotifProvider, NotifRule, Site } from "@/lib/types";
import { IconBell, IconPlus, IconTrash, IconBolt, IconMail, IconLink } from "@/components/icons";

const PROVIDER_KINDS = ["smtp", "resend", "sendgrid", "mailgun", "postmark", "brevo"];

const KIND_LABEL: Record<string, string> = {
  smtp: "SMTP",
  resend: "Resend",
  sendgrid: "SendGrid",
  mailgun: "Mailgun",
  postmark: "Postmark",
  brevo: "Brevo",
};

const EVENT_LABEL: Record<string, string> = {
  site_down: "Site down",
  site_up: "Site back online",
  traffic_spike: "Traffic spike",
};

const EVENT_HINT: Record<string, string> = {
  site_down: "Fires when the uptime check starts failing",
  site_up: "Fires when the site recovers",
  traffic_spike: "Fires when the last hour exceeds the 7-day hourly average by the threshold",
};

const KIND_FIELDS: Record<string, { key: string; label: string; type?: string; placeholder: string }[]> = {
  smtp: [
    { key: "host", label: "Host", placeholder: "smtp.gmail.com" },
    { key: "port", label: "Port", type: "number", placeholder: "587" },
    { key: "user", label: "Username", placeholder: "you@example.com" },
    { key: "pass", label: "Password / App password", type: "password", placeholder: "••••••••" },
    { key: "encryption", label: "Encryption", placeholder: "starttls or ssl" },
  ],
  resend: [{ key: "api_key", label: "API key", type: "password", placeholder: "re_..." }],
  sendgrid: [
    { key: "api_key", label: "API key", type: "password", placeholder: "SG.xxx" },
    { key: "region", label: "Region (global or eu)", placeholder: "global" },
    { key: "from_name", label: "From name", placeholder: "WebStats" },
  ],
  mailgun: [
    { key: "api_key", label: "API key", type: "password", placeholder: "key-..." },
    { key: "domain", label: "Domain", placeholder: "mg.example.com" },
  ],
  postmark: [{ key: "server_token", label: "Server token", type: "password", placeholder: "xxxx-..." }],
  brevo: [
    { key: "api_key", label: "API key", type: "password", placeholder: "xkeysib-..." },
    { key: "from_name", label: "From name", placeholder: "WebStats" },
  ],
};

const MASK = "••••••••";

const inputCls =
  "w-full rounded-lg border border-edge bg-bg px-3 py-2 text-sm text-ink placeholder-faint outline-none transition-colors focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500";
const btnCls =
  "rounded-lg bg-indigo-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-indigo-500 disabled:opacity-50";

function fmtTime(iso: string | null) {
  if (!iso) return "—";
  const d = new Date(iso);
  return isNaN(d.getTime()) ? iso : d.toLocaleString();
}

export default function NotificationsView({
  providers: initialProviders,
  rules: initialRules,
  logs: initialLogs,
  sites,
  token,
}: {
  providers: NotifProvider[];
  rules: NotifRule[];
  logs: NotifLog[];
  sites: Site[];
  token: string;
}) {
  const [providers, setProviders] = useState(initialProviders);
  const [rules, setRules] = useState(initialRules);
  const [logs, setLogs] = useState(initialLogs);
  const [siteId, setSiteId] = useState(sites[0]?.id || "");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  // provider form
  const [pName, setPName] = useState("");
  const [pKind, setPKind] = useState("resend");
  const [pFrom, setPFrom] = useState("");
  const [pCfg, setPCfg] = useState<Record<string, string>>({});
  const [showAdd, setShowAdd] = useState(false);

  // rule form
  const [rEvent, setREvent] = useState("site_down");
  const [rChannel, setRChannel] = useState("webhook");
  const [rProvider, setRProvider] = useState("");
  const [rTarget, setRTarget] = useState("");
  const [rThreshold, setRThreshold] = useState("3");
  const [rCooldown, setRCooldown] = useState("30");
  const [rSecret, setRSecret] = useState("");
  const [showAddRule, setShowAddRule] = useState(false);

  async function addProvider(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      const cfg: Record<string, any> = {};
      for (const f of KIND_FIELDS[pKind]) {
        if (f.key === "port") {
          const n = parseInt(pCfg[f.key] || "0", 10);
          if (n > 0) cfg[f.key] = n;
        } else if (pCfg[f.key] && pCfg[f.key] !== MASK) {
          cfg[f.key] = pCfg[f.key];
        }
      }
      const res = await apiFetch<{ id: string }>("/api/notifications/providers", token, {
        method: "POST",
        body: JSON.stringify({ name: pName, kind: pKind, config: cfg, from_email: pFrom }),
      });
      setProviders((prev) => [
        ...prev,
        { id: res.id, name: pName, kind: pKind, config: cfg, from_email: pFrom, created_at: new Date().toISOString() },
      ]);
      setPName("");
      setPFrom("");
      setPCfg({});
      setShowAdd(false);
    } catch (err: any) {
      setError(err.message);
    }
    setBusy(false);
  }

  async function testProvider(id: string) {
    setError("");
    try {
      await apiFetch(`/api/notifications/providers/${id}/test`, token, { method: "POST" });
      refreshLogs();
    } catch (err: any) {
      setError(err.message);
    }
  }

  async function deleteProvider(id: string, name: string) {
    if (!confirm(`Delete provider "${name}"? Rules using it stop working.`)) return;
    setError("");
    try {
      await apiFetch(`/api/notifications/providers/${id}`, token, { method: "DELETE" });
      setProviders((prev) => prev.filter((p) => p.id !== id));
      setRules((prev) => prev.map((r) => (r.provider_id === id ? { ...r, provider_id: "", provider_name: "" } : r)));
    } catch (err: any) {
      setError(err.message);
    }
  }

  async function addRule(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    if (!siteId) {
      setError("Create a site first");
      return;
    }
    setBusy(true);
    try {
      const params: Record<string, any> = {};
      if (rEvent === "traffic_spike") {
        params.threshold = parseInt(rThreshold || "3", 10) || 3;
        params.cooldown_min = parseInt(rCooldown || "30", 10) || 30;
      }
      if (rChannel === "webhook" && rSecret) params.secret = rSecret;
      const res = await apiFetch<{ id: string }>("/api/notifications/rules", token, {
        method: "POST",
        body: JSON.stringify({
          site_id: siteId,
          event: rEvent,
          channel: rChannel,
          provider_id: rChannel === "email" ? rProvider : "",
          target: rTarget.trim(),
          params,
          enabled: true,
        }),
      });
      const site = sites.find((s) => s.id === siteId);
      setRules((prev) => [
        ...prev,
        {
          id: res.id,
          site_id: siteId,
          site_name: site?.name || "",
          domain: site?.domain || "",
          event: rEvent,
          channel: rChannel,
          target: rTarget.trim(),
          provider_id: rChannel === "email" ? rProvider : "",
          provider_name: rChannel === "email" ? providers.find((p) => p.id === rProvider)?.name || "" : "",
          params,
          enabled: true,
          last_sent_at: null,
        },
      ]);
      setRTarget("");
      setRSecret("");
      setShowAddRule(false);
    } catch (err: any) {
      setError(err.message);
    }
    setBusy(false);
  }

  async function toggleRule(rule: NotifRule) {
    setError("");
    try {
      await apiFetch(`/api/notifications/rules/${rule.id}`, token, {
        method: "PATCH",
        body: JSON.stringify({ enabled: !rule.enabled }),
      });
      setRules((prev) => prev.map((r) => (r.id === rule.id ? { ...r, enabled: !r.enabled } : r)));
    } catch (err: any) {
      setError(err.message);
    }
  }

  async function testRule(rule: NotifRule) {
    setError("");
    try {
      await apiFetch(`/api/notifications/rules/${rule.id}/test`, token, { method: "POST" });
      refreshLogs();
    } catch (err: any) {
      setError(err.message);
    }
  }

  async function deleteRule(rule: NotifRule) {
    if (!confirm(`Delete this ${EVENT_LABEL[rule.event] || rule.event} rule?`)) return;
    setError("");
    try {
      await apiFetch(`/api/notifications/rules/${rule.id}`, token, { method: "DELETE" });
      setRules((prev) => prev.filter((r) => r.id !== rule.id));
    } catch (err: any) {
      setError(err.message);
    }
  }

  async function refreshLogs() {
    try {
      setLogs(await apiFetch<NotifLog[]>("/api/notifications/logs", token));
    } catch {
      // keep stale logs
    }
  }

  const siteRules = rules.filter((r) => r.site_id === siteId);

  return (
    <div className="mt-8 space-y-8">
      {error && (
        <div className="rounded-lg border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-500">
          {error}
        </div>
      )}

      {/* ---------- Providers ---------- */}
      <section>
        <div className="mb-3 flex items-center justify-between">
          <div>
            <h2 className="flex items-center gap-2 text-base font-semibold text-ink">
              <IconMail className="h-4 w-4 text-indigo-500" /> Email providers
            </h2>
            <p className="mt-0.5 text-sm text-faint">
              Connect one or more providers. Rules can pick any of them.
            </p>
          </div>
          <button onClick={() => setShowAdd((v) => !v)} className={btnCls}>
            <span className="flex items-center gap-1.5">
              <IconPlus className="h-4 w-4" /> {showAdd ? "Cancel" : "Add provider"}
            </span>
          </button>
        </div>

        {showAdd && (
          <form onSubmit={addProvider} className="mb-4 space-y-3 rounded-xl border border-edge bg-card p-5">
            <div className="grid gap-3 md:grid-cols-3">
              <input className={inputCls} placeholder="Name (e.g. Work SMTP)" value={pName} onChange={(e) => setPName(e.target.value)} required />
              <select className={inputCls} value={pKind} onChange={(e) => { setPKind(e.target.value); setPCfg({}); }}>
                {PROVIDER_KINDS.map((k) => (
                  <option key={k} value={k}>{KIND_LABEL[k]}</option>
                ))}
              </select>
              <input className={inputCls} type="email" placeholder="From email (e.g. WebStats <noreply@x.com>)" value={pFrom} onChange={(e) => setPFrom(e.target.value)} required />
            </div>
            <div className="grid gap-3 md:grid-cols-3">
              {KIND_FIELDS[pKind].map((f) => (
                <input
                  key={f.key}
                  className={inputCls}
                  type={f.type || "text"}
                  placeholder={f.placeholder}
                  value={pCfg[f.key] || ""}
                  onChange={(e) => setPCfg((prev) => ({ ...prev, [f.key]: e.target.value }))}
                  required={f.key === "api_key" || f.key === "host" || f.key === "domain" || f.key === "server_token"}
                />
              ))}
            </div>
            <button type="submit" disabled={busy} className={btnCls}>Save provider</button>
          </form>
        )}

        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {providers.map((p) => (
            <div key={p.id} className="rounded-xl border border-edge bg-card p-5">
              <div className="flex items-start justify-between">
                <div>
                  <p className="font-medium text-ink">{p.name}</p>
                  <span className="mt-1 inline-block rounded-md bg-indigo-500/10 px-2 py-0.5 text-xs font-medium text-indigo-500">
                    {KIND_LABEL[p.kind] || p.kind}
                  </span>
                </div>
                <div className="flex gap-1">
                  <button
                    title="Send test email"
                    onClick={() => testProvider(p.id)}
                    className="rounded-lg p-2 text-faint transition-colors hover:bg-raised hover:text-ink"
                  >
                    <IconBolt className="h-4 w-4" />
                  </button>
                  <button
                    title="Delete"
                    onClick={() => deleteProvider(p.id, p.name)}
                    className="rounded-lg p-2 text-faint transition-colors hover:bg-raised hover:text-red-500"
                  >
                    <IconTrash className="h-4 w-4" />
                  </button>
                </div>
              </div>
              <p className="mt-2 truncate text-xs text-faint">{p.from_email}</p>
            </div>
          ))}
          {providers.length === 0 && (
            <p className="text-sm text-faint">No providers yet. Add one to receive email alerts.</p>
          )}
        </div>
      </section>

      {/* ---------- Rules ---------- */}
      <section>
        <div className="mb-3 flex items-center justify-between">
          <div>
            <h2 className="flex items-center gap-2 text-base font-semibold text-ink">
              <IconBell className="h-4 w-4 text-indigo-500" /> Alert rules
            </h2>
            <p className="mt-0.5 text-sm text-faint">
              Choose what events notify you and how. Uptime checks run every minute.
            </p>
          </div>
          {sites.length > 0 && (
            <button onClick={() => setShowAddRule((v) => !v)} className={btnCls}>
              <span className="flex items-center gap-1.5">
                <IconPlus className="h-4 w-4" /> {showAddRule ? "Cancel" : "Add rule"}
              </span>
            </button>
          )}
        </div>

        {sites.length === 0 && (
          <p className="text-sm text-faint">Create a site first to configure alert rules.</p>
        )}

        {sites.length > 0 && (
          <select className={`${inputCls} mb-4 max-w-xs`} value={siteId} onChange={(e) => setSiteId(e.target.value)}>
            {sites.map((s) => (
              <option key={s.id} value={s.id}>{s.name} — {s.domain || "no domain"}</option>
            ))}
          </select>
        )}

        {showAddRule && sites.length > 0 && (
          <form onSubmit={addRule} className="mb-4 space-y-3 rounded-xl border border-edge bg-card p-5">
            <div className="grid gap-3 md:grid-cols-3">
              <select className={inputCls} value={rEvent} onChange={(e) => setREvent(e.target.value)}>
                <option value="site_down">Site down</option>
                <option value="site_up">Site back online</option>
                <option value="traffic_spike">Traffic spike</option>
              </select>
              <select
                className={inputCls}
                value={rChannel}
                onChange={(e) => { setRChannel(e.target.value); setRProvider(providers[0]?.id || ""); }}
              >
                <option value="webhook">Webhook</option>
                <option value="email">Email</option>
              </select>
              {rChannel === "email" ? (
                <select className={inputCls} value={rProvider} onChange={(e) => setRProvider(e.target.value)} required>
                  <option value="">Choose provider</option>
                  {providers.map((p) => (
                    <option key={p.id} value={p.id}>{p.name}</option>
                  ))}
                </select>
              ) : (
                <input className={inputCls} placeholder="https://hooks.example.com/alert" value={rTarget} onChange={(e) => setRTarget(e.target.value)} required />
              )}
            </div>
            {rChannel === "email" && (
              <input className={inputCls} type="email" placeholder="Recipient email" value={rTarget} onChange={(e) => setRTarget(e.target.value)} required />
            )}
            {rChannel === "webhook" && (
              <input className={inputCls} placeholder="Shared secret (optional, sent as X-Webstats-Secret)" value={rSecret} onChange={(e) => setRSecret(e.target.value)} />
            )}
            {rEvent === "traffic_spike" && (
              <div className="grid gap-3 md:grid-cols-2">
                <input className={inputCls} type="number" min={1} placeholder="Threshold (x average, default 3)" value={rThreshold} onChange={(e) => setRThreshold(e.target.value)} />
                <input className={inputCls} type="number" min={1} placeholder="Cooldown minutes (default 30)" value={rCooldown} onChange={(e) => setRCooldown(e.target.value)} />
              </div>
            )}
            <p className="text-xs text-faint">{EVENT_HINT[rEvent]}</p>
            <button type="submit" disabled={busy} className={btnCls}>Save rule</button>
          </form>
        )}

        <div className="space-y-3">
          {siteRules.map((r) => (
            <div key={r.id} className="flex flex-wrap items-center gap-3 rounded-xl border border-edge bg-card px-4 py-3">
              <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-indigo-500/10 text-indigo-500">
                {r.channel === "email" ? <IconMail className="h-4 w-4" /> : <IconLink className="h-4 w-4" />}
              </span>
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium text-ink">
                  {EVENT_LABEL[r.event] || r.event}
                  <span className="ml-2 rounded-md bg-raised px-2 py-0.5 text-xs font-normal text-soft">
                    {r.channel}
                  </span>
                  {r.channel === "email" && r.provider_name && (
                    <span className="ml-2 rounded-md bg-raised px-2 py-0.5 text-xs font-normal text-soft">
                      via {r.provider_name}
                    </span>
                  )}
                </p>
                <p className="truncate text-xs text-faint">
                  {r.channel === "email" ? `To ${r.target}` : r.target}
                  {r.event === "traffic_spike" && ` · ${r.params.threshold || 3}x avg · cooldown ${r.params.cooldown_min || 30}m`}
                  {r.last_sent_at && ` · last sent ${fmtTime(r.last_sent_at)}`}
                </p>
              </div>
              <button
                onClick={() => toggleRule(r)}
                className={`relative h-5 w-9 shrink-0 rounded-full transition-colors ${r.enabled ? "bg-indigo-600" : "bg-edge"}`}
                title={r.enabled ? "Disable" : "Enable"}
              >
                <span className={`absolute top-0.5 h-4 w-4 rounded-full bg-white transition-all ${r.enabled ? "left-[18px]" : "left-0.5"}`} />
              </button>
              <button
                title="Send test alert"
                onClick={() => testRule(r)}
                className="rounded-lg px-2 py-1.5 text-xs font-medium text-indigo-500 transition-colors hover:bg-raised"
              >
                Test now
              </button>
              <button
                title="Delete rule"
                onClick={() => deleteRule(r)}
                className="rounded-lg p-2 text-faint transition-colors hover:bg-raised hover:text-red-500"
              >
                <IconTrash className="h-4 w-4" />
              </button>
            </div>
          ))}
          {siteRules.length === 0 && sites.length > 0 && (
            <p className="text-sm text-faint">No rules for this site yet.</p>
          )}
        </div>
      </section>

      {/* ---------- Logs ---------- */}
      <section>
        <h2 className="mb-3 text-base font-semibold text-ink">Delivery log</h2>
        <div className="overflow-x-auto rounded-xl border border-edge">
          <table className="w-full text-left text-sm">
            <thead>
              <tr className="border-b border-edge text-xs uppercase tracking-wide text-faint">
                <th className="px-4 py-2.5 font-medium">Time</th>
                <th className="px-4 py-2.5 font-medium">Site</th>
                <th className="px-4 py-2.5 font-medium">Event</th>
                <th className="px-4 py-2.5 font-medium">Channel</th>
                <th className="px-4 py-2.5 font-medium">Status</th>
                <th className="px-4 py-2.5 font-medium">Detail</th>
              </tr>
            </thead>
            <tbody>
              {logs.map((l) => (
                <tr key={l.id} className="border-b border-edge/60 last:border-0">
                  <td className="whitespace-nowrap px-4 py-2.5 text-faint">{fmtTime(l.created_at)}</td>
                  <td className="px-4 py-2.5 text-ink">{l.site_name || l.domain || "—"}</td>
                  <td className="px-4 py-2.5 text-ink">{EVENT_LABEL[l.event] || l.event}</td>
                  <td className="px-4 py-2.5 text-soft">{l.channel}</td>
                  <td className="px-4 py-2.5">
                    <span className={`rounded-md px-2 py-0.5 text-xs font-medium ${l.status === "ok" ? "bg-emerald-500/10 text-emerald-500" : "bg-red-500/10 text-red-500"}`}>
                      {l.status}
                    </span>
                  </td>
                  <td className="max-w-[280px] truncate px-4 py-2.5 text-faint" title={l.detail}>{l.detail || "—"}</td>
                </tr>
              ))}
              {logs.length === 0 && (
                <tr><td colSpan={6} className="px-4 py-6 text-center text-faint">No deliveries yet.</td></tr>
              )}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}