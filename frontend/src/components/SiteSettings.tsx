"use client";

import { useState } from "react";
import Link from "next/link";
import { apiFetch } from "@/lib/auth";
import type { FunnelConfig, Invite, Member, Monitor, MonitorCheck, Site, SiteSettings as SiteSettingsT, SslResult } from "@/lib/types";
import {
  IconArrowLeft,
  IconCheck,
  IconCopy,
  IconShieldCheck,
  IconShieldX,
  IconUsers,
  IconTrash,
  IconPlus,
  IconShield,
  IconGlobe,
  IconPulse,
  IconRefresh,
} from "@/components/icons";

const trackerUrl = () =>
  process.env.NEXT_PUBLIC_TRACKER_URL ||
  (typeof window !== "undefined" ? window.location.origin : "");

const PALETTE = [
  "#ef4444",
  "#3b82f6",
  "#10b981",
  "#f59e0b",
  "#8b5cf6",
  "#ec4899",
  "#06b6d4",
  "#84cc16",
  "#f97316",
  "#0ea5e9",
];

export default function SiteSettings({
  site,
  token,
  loadError,
  initialMembers = [],
  initialInvites = [],
  initialSettings = null,
  initialFunnels = [],
  initialMonitors = [],
}: {
  site: Site | null;
  token: string;
  loadError: string;
  initialMembers?: Member[];
  initialInvites?: Invite[];
  initialSettings?: SiteSettingsT | null;
  initialFunnels?: FunnelConfig[];
  initialMonitors?: Monitor[];
}) {
  const [name, setName] = useState(site?.name || "");
  const [domain, setDomain] = useState(site?.domain || "");
  const [color, setColor] = useState(site?.color || PALETTE[0]);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [saveError, setSaveError] = useState("");
  const [copied, setCopied] = useState(false);

  const [ssl, setSsl] = useState<SslResult | null>(null);
  const [checking, setChecking] = useState(false);
  const [sslError, setSslError] = useState("");

  const [members, setMembers] = useState(initialMembers);
  const [invites, setInvites] = useState(initialInvites);
  const [invEmail, setInvEmail] = useState("");
  const [invRole, setInvRole] = useState("viewer");
  const [teamMsg, setTeamMsg] = useState("");
  const [newInvite, setNewInvite] = useState("");

  const [funnels, setFunnels] = useState<FunnelConfig[]>(initialFunnels);
  const [funnelInput, setFunnelInput] = useState(
    initialFunnels.map((f) => f.label).join(", "),
  );
  const [funnelMsg, setFunnelMsg] = useState("");
  const [savingFunnel, setSavingFunnel] = useState(false);

  const [monitors, setMonitors] = useState<Monitor[]>(initialMonitors);
  const [monUrl, setMonUrl] = useState("");
  const [monInterval, setMonInterval] = useState(60);
  const [monMsg, setMonMsg] = useState("");
  const [checks, setChecks] = useState<Record<string, MonitorCheck[]>>({});

  const [publicEnabled, setPublicEnabled] = useState(
    initialSettings?.public_enabled ?? false,
  );
  const [publicToken, setPublicToken] = useState(initialSettings?.public_token || "");
  const [publicMsg, setPublicMsg] = useState("");
  const origin = typeof window !== "undefined" ? window.location.origin : "";

  const [settings, setSettings] = useState<SiteSettingsT | null>(initialSettings);
  const [settingsMsg, setSettingsMsg] = useState("");
  const [savingSettings, setSavingSettings] = useState(false);
  const [ipHashing, setIpHashing] = useState(initialSettings?.ip_hashing ?? true);
  const [retention, setRetention] = useState(String(initialSettings?.retention_days ?? 0));

  if (!site) {
    return (
      <div className="rounded-xl border border-red-900/60 bg-red-950/40 p-6 text-sm text-red-300">
        {loadError || "Site not found"}
      </div>
    );
  }

  async function save(e: React.FormEvent) {
    e.preventDefault();
    if (!site) return;
    setSaving(true);
    setSaveError("");
    setSaved(false);
    try {
      await apiFetch(`/api/sites/${site.id}`, token, {
        method: "PATCH",
        body: JSON.stringify({ name, domain, color }),
      });
      setSaved(true);
    } catch (err: any) {
      setSaveError(err.message);
    }
    setSaving(false);
  }

  async function checkSsl() {
    if (!site) return;
    setChecking(true);
    setSsl(null);
    setSslError("");
    try {
      const res = await apiFetch<SslResult>(
        `/api/sites/${site.id}/ssl-check`,
        token,
      );
      setSsl(res);
    } catch (err: any) {
      setSslError(err.message);
    }
    setChecking(false);
  }

  const installCode = `<script async src="${trackerUrl()}/track.js" data-site="${site.site_key}"></script>`;

  async function sendInvite(e: React.FormEvent) {
    e.preventDefault();
    if (!site) return;
    setTeamMsg("");
    setNewInvite("");
    try {
      const res = await apiFetch<Invite | { added: boolean; email: string }>(
        `/api/sites/${site.id}/invites`,
        token,
        { method: "POST", body: JSON.stringify({ email: invEmail, role: invRole }) },
      );
      if ("added" in res) {
        setTeamMsg(`Member added: ${res.email}`);
        const ms = await apiFetch<Member[]>(`/api/sites/${site.id}/members`, token);
        setMembers(ms);
      } else {
        setNewInvite(`${location.origin}${res.invite_url}`);
        setInvites((prev) => [...prev, res]);
      }
      setInvEmail("");
    } catch (err: any) {
      setTeamMsg(err.message);
    }
  }

  async function deleteInvite(id: string) {
    if (!site) return;
    setTeamMsg("");
    try {
      await apiFetch(`/api/sites/${site.id}/invites/${id}`, token, { method: "DELETE" });
      setInvites((prev) => prev.filter((i) => i.id !== id));
    } catch (err: any) {
      setTeamMsg(err.message);
    }
  }

  async function removeMember(userId: string) {
    if (!site) return;
    if (!confirm("Remove this member from the site?")) return;
    setTeamMsg("");
    try {
      await apiFetch(`/api/sites/${site.id}/members/${userId}`, token, { method: "DELETE" });
      setMembers((prev) => prev.filter((m) => m.user_id !== userId));
    } catch (err: any) {
      setTeamMsg(err.message);
    }
  }

  async function saveFunnel(e: React.FormEvent) {
    e.preventDefault();
    if (!site) return;
    setSavingFunnel(true);
    setFunnelMsg("");
    const paths = funnelInput.split(",").map((x) => x.trim()).filter(Boolean);
    if (paths.length < 2) {
      setFunnelMsg("Enter at least 2 steps separated by commas");
      setSavingFunnel(false);
      return;
    }
    try {
      await apiFetch(`/api/sites/${site.id}/funnels`, token, {
        method: "PUT",
        body: JSON.stringify({ paths }),
      });
      setFunnelMsg("Funnel updated");
      const res = await apiFetch<FunnelConfig[]>(`/api/sites/${site.id}/funnels`, token);
      setFunnels(res);
    } catch (err: any) {
      setFunnelMsg(err.message);
    }
    setSavingFunnel(false);
  }

  async function addMonitor(e: React.FormEvent) {
    e.preventDefault();
    if (!site) return;
    setMonMsg("");
    try {
      await apiFetch(`/api/sites/${site.id}/monitors`, token, {
        method: "POST",
        body: JSON.stringify({ url: monUrl, interval_seconds: monInterval }),
      });
      setMonUrl("");
      const res = await apiFetch<Monitor[]>(`/api/sites/${site.id}/monitors`, token);
      setMonitors(res);
    } catch (err: any) {
      setMonMsg(err.message);
    }
  }

  async function toggleMonitor(m: Monitor) {
    if (!site) return;
    setMonMsg("");
    try {
      await apiFetch(`/api/sites/${site.id}/monitors/${m.id}`, token, {
        method: "PATCH",
        body: JSON.stringify({ enabled: !m.enabled }),
      });
      setMonitors((prev) => prev.map((x) => (x.id === m.id ? { ...x, enabled: !m.enabled } : x)));
    } catch (err: any) {
      setMonMsg(err.message);
    }
  }

  async function deleteMonitor(m: Monitor) {
    if (!site) return;
    if (!confirm("Delete this monitor?")) return;
    setMonMsg("");
    try {
      await apiFetch(`/api/sites/${site.id}/monitors/${m.id}`, token, { method: "DELETE" });
      setMonitors((prev) => prev.filter((x) => x.id !== m.id));
    } catch (err: any) {
      setMonMsg(err.message);
    }
  }

  async function loadChecks(m: Monitor) {
    if (!site) return;
    if (checks[m.id]) {
      const next = { ...checks };
      delete next[m.id];
      setChecks(next);
      return;
    }
    try {
      const res = await apiFetch<MonitorCheck[]>(
        `/api/sites/${site.id}/monitors/${m.id}/checks?limit=12`,
        token,
      );
      setChecks((prev) => ({ ...prev, [m.id]: res }));
    } catch {
      setChecks((prev) => ({ ...prev, [m.id]: [] }));
    }
  }

  async function savePublic(enabled: boolean) {
    if (!site) return;
    setPublicMsg("");
    try {
      let tok = publicToken;
      if (enabled && !tok) {
        tok = Math.random().toString(36).slice(2) + Math.random().toString(36).slice(2);
      }
      const res = await apiFetch<SiteSettingsT>(`/api/sites/${site.id}/settings`, token, {
        method: "PATCH",
        body: JSON.stringify({ public_enabled: enabled, public_token: tok || undefined }),
      });
      setPublicEnabled(res.public_enabled);
      setPublicToken(res.public_token || "");
      setSettings(res);
      setPublicMsg(res.public_enabled ? "Public dashboard enabled" : "Public dashboard disabled");
    } catch (err: any) {
      setPublicMsg(err.message);
    }
  }

  async function regenPublic() {
    if (!site) return;
    setPublicMsg("");
    try {
      const tok = Math.random().toString(36).slice(2) + Math.random().toString(36).slice(2);
      const res = await apiFetch<SiteSettingsT>(`/api/sites/${site.id}/settings`, token, {
        method: "PATCH",
        body: JSON.stringify({ public_token: tok }),
      });
      setPublicToken(res.public_token || "");
      setPublicMsg("New public link generated");
    } catch (err: any) {
      setPublicMsg(err.message);
    }
  }

  async function saveSettings(e: React.FormEvent) {
    e.preventDefault();
    if (!site) return;
    setSavingSettings(true);
    setSettingsMsg("");
    try {
      const res = await apiFetch<SiteSettingsT>(`/api/sites/${site.id}/settings`, token, {
        method: "PATCH",
        body: JSON.stringify({ ip_hashing: ipHashing, retention_days: parseInt(retention || "0", 10) }),
      });
      setSettings(res);
      setSettingsMsg("Privacy settings saved");
    } catch (err: any) {
      setSettingsMsg(err.message);
    }
    setSavingSettings(false);
  }

  const inputCls =
    "w-full rounded-lg border border-edge bg-bg px-3 py-2 text-sm text-ink placeholder-faint outline-none transition-colors focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500";

  return (
    <div className="space-y-6">
      <header>
        <Link
          href={`/sites/${site.id}`}
          className="inline-flex items-center gap-1.5 text-sm text-faint transition-colors hover:text-ink"
        >
          <IconArrowLeft className="h-3.5 w-3.5" />
          Back to stats
        </Link>
        <h1 className="mt-1 text-2xl font-semibold tracking-tight text-ink">
          Settings — {site.name}
        </h1>
      </header>

      <form
        onSubmit={save}
        className="space-y-5 rounded-xl border border-edge bg-card p-6"
      >
        <div>
          <label className="text-xs font-medium text-soft">Site name</label>
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            className={`${inputCls} mt-1.5`}
          />
        </div>
        <div>
          <label className="text-xs font-medium text-soft">Domain</label>
          <input
            value={domain}
            onChange={(e) => setDomain(e.target.value)}
            className={`${inputCls} mt-1.5`}
            placeholder="example.com"
          />
          <p className="mt-1.5 text-xs text-faint">
            Used for SSL checking and shown in the dashboard.
          </p>
        </div>
        <div>
          <label className="text-xs font-medium text-soft">Chart color</label>
          <div className="mt-2 flex flex-wrap gap-2.5">
            {PALETTE.map((c) => (
              <button
                key={c}
                type="button"
                onClick={() => setColor(c)}
                title={c}
                className={`flex h-8 w-8 items-center justify-center rounded-full transition-transform ${
                  color === c ? "ring-2 ring-offset-2 ring-indigo-500 ring-offset-card" : "hover:scale-110"
                }`}
                style={{ background: c }}
              >
                {color === c && (
                  <IconCheck className="h-4 w-4 text-white" />
                )}
              </button>
            ))}
          </div>
        </div>

        {saveError && <p className="text-sm text-red-400">{saveError}</p>}
        <div className="flex items-center gap-3">
          <button
            type="submit"
            disabled={saving}
            className="rounded-lg bg-indigo-600 px-4 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-indigo-500 disabled:opacity-60"
          >
            {saving ? "Saving..." : "Save changes"}
          </button>
          {saved && (
            <span className="flex items-center gap-1.5 text-sm text-emerald-500">
              <IconCheck className="h-4 w-4" />
              Saved
            </span>
          )}
        </div>
      </form>

      <section className="rounded-xl border border-edge bg-card p-6">
        <h2 className="text-sm font-semibold text-ink">Tracking script</h2>
        <div className="mt-3 flex items-center gap-2">
          <code className="min-w-0 flex-1 truncate rounded-md border border-edge bg-raised px-2.5 py-2 font-mono text-xs text-soft">
            {installCode}
          </code>
          <button
            onClick={() => {
              navigator.clipboard.writeText(installCode);
              setCopied(true);
              setTimeout(() => setCopied(false), 1500);
            }}
            className="flex items-center gap-1.5 rounded-md border border-edge px-2.5 py-2 text-xs text-soft transition-colors hover:bg-raised"
          >
            {copied ? (
              <IconCheck className="h-3.5 w-3.5 text-emerald-500" />
            ) : (
              <IconCopy className="h-3.5 w-3.5" />
            )}
            {copied ? "Copied" : "Copy"}
          </button>
        </div>
        <p className="mt-2.5 text-xs text-faint">
          Site key: <code className="text-indigo-500">{site.site_key}</code>
        </p>
      </section>

      <section className="rounded-xl border border-edge bg-card p-6">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-sm font-semibold text-ink">SSL certificate</h2>
            <p className="mt-0.5 text-xs text-faint">
              Checks the HTTPS certificate of the domain you set above.
            </p>
          </div>
          <button
            onClick={checkSsl}
            disabled={checking || !domain}
            className="rounded-lg border border-edge px-4 py-2 text-sm font-medium text-ink transition-colors hover:bg-raised disabled:cursor-not-allowed disabled:opacity-50"
          >
            {checking ? "Checking..." : "Check SSL"}
          </button>
        </div>

        {sslError && (
          <p className="mt-4 text-sm text-red-400">SSL check failed: {sslError}</p>
        )}
        {ssl && (
          <div
            className={`mt-4 rounded-lg border p-4 text-sm ${
              ssl.valid
                ? "border-emerald-500/40 bg-emerald-500/10"
                : "border-red-500/40 bg-red-500/10"
            }`}
          >
            <p
              className={`flex items-center gap-2 font-medium ${
                ssl.valid ? "text-emerald-500" : "text-red-400"
              }`}
            >
              {ssl.valid ? (
                <IconShieldCheck className="h-4 w-4" />
              ) : (
                <IconShieldX className="h-4 w-4" />
              )}
              {ssl.valid ? "SSL is valid" : "SSL check failed"}
            </p>
            <dl className="mt-3 grid gap-1.5 text-xs">
              <div className="flex gap-2">
                <dt className="w-20 shrink-0 text-faint">URL</dt>
                <dd className="text-soft">{ssl.url}</dd>
              </div>
              {ssl.valid ? (
                <>
                  <div className="flex gap-2">
                    <dt className="w-20 shrink-0 text-faint">Issuer</dt>
                    <dd className="text-soft">{ssl.issuer}</dd>
                  </div>
                  <div className="flex gap-2">
                    <dt className="w-20 shrink-0 text-faint">Subject</dt>
                    <dd className="text-soft">{ssl.subject}</dd>
                  </div>
                  <div className="flex gap-2">
                    <dt className="w-20 shrink-0 text-faint">Expires</dt>
                    <dd className="text-soft">
                      {new Date(ssl.expires_at).toLocaleDateString("en-US", {
                        year: "numeric",
                        month: "short",
                        day: "numeric",
                      })}{" "}
                      ({ssl.days_left} days)
                    </dd>
                  </div>
                </>
              ) : (
                <div className="flex gap-2">
                  <dt className="w-20 shrink-0 text-faint">Error</dt>
                  <dd className="text-soft">{ssl.error || "Unknown error"}</dd>
                </div>
              )}
            </dl>
          </div>
        )}
      </section>

      <section className="rounded-xl border border-edge bg-card p-6">
        <h2 className="flex items-center gap-2 text-sm font-semibold text-ink">
          <IconUsers className="h-4 w-4 text-indigo-500" />
          Team
        </h2>
        <p className="mt-0.5 text-xs text-faint">
          Invite people to view this site. Invites expire after 7 days.
        </p>

        <form onSubmit={sendInvite} className="mt-4 flex flex-wrap items-center gap-2">
          <input
            type="email"
            value={invEmail}
            onChange={(e) => setInvEmail(e.target.value)}
            placeholder="teammate@example.com"
            required
            className="min-w-52 flex-1 rounded-lg border border-edge bg-bg px-3 py-2 text-sm text-ink placeholder-faint outline-none focus:border-indigo-500"
          />
          <select
            value={invRole}
            onChange={(e) => setInvRole(e.target.value)}
            className="rounded-lg border border-edge bg-bg px-2 py-2 text-sm text-ink outline-none focus:border-indigo-500"
          >
            <option value="viewer">Viewer</option>
            <option value="editor">Editor</option>
          </select>
          <button
            type="submit"
            className="flex items-center gap-1.5 rounded-lg bg-indigo-600 px-3.5 py-2 text-sm font-medium text-white transition-colors hover:bg-indigo-500"
          >
            <IconPlus className="h-4 w-4" /> Invite
          </button>
        </form>

        {teamMsg && <p className="mt-3 text-sm text-emerald-500">{teamMsg}</p>}
        {newInvite && (
          <div className="mt-3 rounded-lg border border-indigo-500/30 bg-indigo-500/10 p-3">
            <p className="text-xs font-medium text-indigo-400">Share this invite link:</p>
            <div className="mt-1.5 flex items-center gap-2">
              <code className="min-w-0 flex-1 truncate font-mono text-xs text-soft">{newInvite}</code>
              <button
                onClick={() => {
                  navigator.clipboard.writeText(newInvite);
                  setNewInvite("");
                  setTeamMsg("Invite link copied");
                }}
                className="rounded-md border border-edge px-2 py-1 text-xs text-soft transition-colors hover:bg-raised"
              >
                Copy
              </button>
            </div>
          </div>
        )}

        <div className="mt-4 space-y-2">
          {members.map((m) => (
            <div key={m.user_id} className="flex items-center gap-3 rounded-lg border border-edge/60 px-3 py-2.5">
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium text-ink">
                  {m.name || m.email}
                  <span className="ml-2 rounded-md bg-raised px-1.5 py-0.5 text-[11px] font-normal text-soft">
                    {m.is_owner ? "owner" : m.role}
                  </span>
                </p>
                <p className="truncate text-xs text-faint">{m.email}</p>
              </div>
              {!m.is_owner && (
                <button
                  onClick={() => removeMember(m.user_id)}
                  className="rounded-lg p-2 text-faint transition-colors hover:bg-raised hover:text-red-500"
                  title="Remove member"
                >
                  <IconTrash className="h-4 w-4" />
                </button>
              )}
            </div>
          ))}
          {members.length === 0 && <p className="text-xs text-faint">No members yet.</p>}
        </div>

        {invites.length > 0 && (
          <div className="mt-4 border-t border-edge pt-3">
            <h3 className="text-xs font-medium uppercase tracking-wide text-faint">Pending invites</h3>
            <div className="mt-2 space-y-2">
              {invites.map((i) => (
                <div key={i.id} className="flex items-center gap-3 rounded-lg border border-edge/60 px-3 py-2">
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm text-ink">{i.email}</p>
                    <p className="text-xs text-faint">
                      {i.role} · expires {new Date(i.expires_at).toLocaleDateString("en-US", { month: "short", day: "numeric" })}
                    </p>
                  </div>
                  <button
                    onClick={() => deleteInvite(i.id)}
                    className="rounded-lg p-2 text-faint transition-colors hover:bg-raised hover:text-red-500"
                    title="Revoke invite"
                  >
                    <IconTrash className="h-4 w-4" />
                  </button>
                </div>
              ))}
            </div>
          </div>
        )}
      </section>

      <section className="rounded-xl border border-edge bg-card p-6">
        <h2 className="flex items-center gap-2 text-sm font-semibold text-ink">
          <IconShield className="h-4 w-4 text-indigo-500" />
          Privacy
        </h2>
        <p className="mt-0.5 text-xs text-faint">
          Control how visitor data is stored and how long it is kept.
        </p>

        <form onSubmit={saveSettings} className="mt-4 space-y-4">
          <label className="flex items-start gap-3">
            <input
              type="checkbox"
              checked={ipHashing}
              onChange={(e) => setIpHashing(e.target.checked)}
              className="mt-0.5 h-4 w-4 accent-indigo-600"
            />
            <span>
              <span className="block text-sm text-ink">Hash visitor IPs</span>
              <span className="block text-xs text-faint">
                Store a salted hash instead of the raw IP address. Disable to not store any IP data at all.
              </span>
            </span>
          </label>

          <label className="block">
            <span className="text-sm text-ink">Data retention (days)</span>
            <input
              type="number"
              min={0}
              max={730}
              value={retention}
              onChange={(e) => setRetention(e.target.value)}
              className="mt-1.5 w-32 rounded-lg border border-edge bg-bg px-3 py-2 text-sm text-ink outline-none focus:border-indigo-500"
            />
            <span className="block text-xs text-faint">
              0 = keep forever. Old pageviews and events are deleted automatically.
            </span>
          </label>

          <label className="block">
            <span className="text-sm text-ink">Visitor opt-out</span>
            <code className="mt-1.5 block truncate rounded-md border border-edge bg-raised px-2.5 py-2 font-mono text-xs text-soft">
              document.cookie = "webstats_optout=1; path=/; max-age=31536000"
            </code>
            <span className="block text-xs text-faint">
              Visitors with this cookie are not tracked. They can also call{" "}
              <code className="text-indigo-500">webstats.setOptout(true)</code>.
            </span>
          </label>

          {settingsMsg && <p className="text-sm text-emerald-500">{settingsMsg}</p>}
          <button
            type="submit"
            disabled={savingSettings}
            className="rounded-lg bg-indigo-600 px-4 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-indigo-500 disabled:opacity-60"
          >
            {savingSettings ? "Saving..." : "Save privacy settings"}
          </button>
        </form>
      </section>

      <section className="rounded-xl border border-edge bg-card p-6">
        <h2 className="flex items-center gap-2 text-sm font-semibold text-ink">
          <IconGlobe className="h-4 w-4 text-indigo-500" />
          Public dashboard
        </h2>
        <p className="mt-0.5 text-xs text-faint">
          Share a read-only version of this site's stats without login.
        </p>

        <label className="mt-4 flex items-start gap-3">
          <input
            type="checkbox"
            checked={publicEnabled}
            onChange={(e) => savePublic(e.target.checked)}
            className="mt-0.5 h-4 w-4 accent-indigo-600"
          />
          <span>
            <span className="block text-sm text-ink">Enable public dashboard</span>
            <span className="block text-xs text-faint">
              Anyone with the link can view traffic, pages, sources, funnels and uptime.
            </span>
          </span>
        </label>

        {publicEnabled && publicToken && (
          <div className="mt-3 rounded-lg border border-indigo-500/30 bg-indigo-500/10 p-3">
            <p className="text-xs font-medium text-indigo-400">Public link:</p>
            <div className="mt-1.5 flex items-center gap-2">
              <code className="min-w-0 flex-1 truncate font-mono text-xs text-soft">
                {`${origin}/public/${publicToken}`}
              </code>
              <button
                onClick={() => {
                  navigator.clipboard.writeText(`${origin}/public/${publicToken}`);
                  setPublicMsg("Link copied");
                }}
                className="rounded-md border border-edge px-2 py-1 text-xs text-soft transition-colors hover:bg-raised"
              >
                Copy
              </button>
              <button
                onClick={regenPublic}
                className="rounded-md border border-edge px-2 py-1 text-xs text-soft transition-colors hover:bg-raised"
                title="Generate a new link"
              >
                <IconRefresh className="h-3.5 w-3.5" />
              </button>
            </div>
          </div>
        )}
        {publicMsg && <p className="mt-3 text-sm text-emerald-500">{publicMsg}</p>}
      </section>

      <section className="rounded-xl border border-edge bg-card p-6">
        <h2 className="flex items-center gap-2 text-sm font-semibold text-ink">
          <IconPulse className="h-4 w-4 text-indigo-500" />
          Funnel
        </h2>
        <p className="mt-0.5 text-xs text-faint">
          Define the page steps of your conversion funnel. Shown on the stats page and public dashboard.
        </p>
        <form onSubmit={saveFunnel} className="mt-4 space-y-3">
          <input
            value={funnelInput}
            onChange={(e) => setFunnelInput(e.target.value)}
            placeholder="Steps, e.g. /, /pricing, /signup"
            className="w-full rounded-lg border border-edge bg-bg px-3 py-2 text-sm text-ink placeholder-faint outline-none focus:border-indigo-500"
          />
          <div className="flex items-center gap-3">
            <button
              type="submit"
              disabled={savingFunnel}
              className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-indigo-500 disabled:opacity-60"
            >
              {savingFunnel ? "Saving..." : "Save funnel"}
            </button>
            {funnels.length > 0 && (
              <span className="text-xs text-faint">
                {funnels.length} steps saved
              </span>
            )}
          </div>
        </form>
        {funnelMsg && (
          <p className={`mt-2 text-sm ${funnelMsg === "Funnel updated" ? "text-emerald-500" : "text-red-400"}`}>
            {funnelMsg}
          </p>
        )}
      </section>

      <section className="rounded-xl border border-edge bg-card p-6">
        <h2 className="flex items-center gap-2 text-sm font-semibold text-ink">
          <IconShield className="h-4 w-4 text-indigo-500" />
          Uptime monitors
        </h2>
        <p className="mt-0.5 text-xs text-faint">
          Check your site's availability periodically. Results are public via the status page.
        </p>
        <form onSubmit={addMonitor} className="mt-4 flex flex-wrap items-center gap-2">
          <input
            value={monUrl}
            onChange={(e) => setMonUrl(e.target.value)}
            placeholder="https://example.com"
            required
            className="min-w-56 flex-1 rounded-lg border border-edge bg-bg px-3 py-2 text-sm text-ink placeholder-faint outline-none focus:border-indigo-500"
          />
          <select
            value={monInterval}
            onChange={(e) => setMonInterval(parseInt(e.target.value, 10))}
            className="rounded-lg border border-edge bg-bg px-2 py-2 text-sm text-ink outline-none focus:border-indigo-500"
          >
            <option value={60}>Every 1 min</option>
            <option value={300}>Every 5 min</option>
            <option value={900}>Every 15 min</option>
          </select>
          <button
            type="submit"
            className="flex items-center gap-1.5 rounded-lg bg-indigo-600 px-3.5 py-2 text-sm font-medium text-white transition-colors hover:bg-indigo-500"
          >
            <IconPlus className="h-4 w-4" /> Add
          </button>
        </form>
        {monMsg && <p className="mt-3 text-sm text-red-400">{monMsg}</p>}

        <div className="mt-4 space-y-2">
          {monitors.length === 0 && <p className="text-xs text-faint">No monitors yet.</p>}
          {monitors.map((m) => (
            <div key={m.id} className="rounded-lg border border-edge/60 px-3 py-2.5">
              <div className="flex items-center gap-3">
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium text-ink">{m.url}</p>
                  <p className="text-xs text-faint">
                    {m.last_ok === null || m.last_ok === undefined
                      ? "waiting for first check"
                      : m.last_ok
                        ? `up · ${m.uptime_pct.toFixed(2)}% uptime`
                        : `down (HTTP ${m.last_status || "error"}) · ${m.uptime_pct.toFixed(2)}% uptime`}
                  </p>
                </div>
                <span
                  className={`flex h-2.5 w-2.5 rounded-full ${
                    m.last_ok === null || m.last_ok === undefined
                      ? "bg-soft/40"
                      : m.last_ok
                        ? "bg-emerald-500"
                        : "bg-red-500"
                  }`}
                />
                <button
                  onClick={() => toggleMonitor(m)}
                  className={`rounded-md px-2 py-1 text-xs transition-colors ${
                    m.enabled
                      ? "bg-raised text-soft hover:text-ink"
                      : "bg-red-500/10 text-red-400 hover:bg-red-500/20"
                  }`}
                >
                  {m.enabled ? "Pause" : "Resume"}
                </button>
                <button
                  onClick={() => loadChecks(m)}
                  className="rounded-md border border-edge px-2 py-1 text-xs text-soft transition-colors hover:bg-raised"
                >
                  {checks[m.id] ? "Hide" : "Checks"}
                </button>
                <button
                  onClick={() => deleteMonitor(m)}
                  className="rounded-lg p-2 text-faint transition-colors hover:bg-raised hover:text-red-500"
                  title="Delete monitor"
                >
                  <IconTrash className="h-4 w-4" />
                </button>
              </div>
              {checks[m.id] && (
                <div className="mt-2 flex flex-wrap gap-1.5">
                  {checks[m.id].length === 0 && <span className="text-xs text-faint">No checks yet.</span>}
                  {[...checks[m.id]].reverse().map((c, i) => (
                    <span
                      key={i}
                      className={`rounded-md px-2 py-0.5 text-[11px] ${
                        c.ok ? "bg-emerald-500/10 text-emerald-500" : "bg-red-500/10 text-red-500"
                      }`}
                      title={`${c.checked_at} · ${c.latency_ms}ms`}
                    >
                      {c.ok ? `${c.status_code} · ${c.latency_ms}ms` : "down"}
                    </span>
                  ))}
                </div>
              )}
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}