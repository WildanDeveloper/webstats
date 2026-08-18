"use client";

import { useState } from "react";
import Link from "next/link";
import { apiFetch } from "@/lib/auth";
import type { Site, SslResult } from "@/lib/types";
import {
  IconArrowLeft,
  IconCheck,
  IconCopy,
  IconShieldCheck,
  IconShieldX,
} from "@/components/icons";

const trackerUrl =
  process.env.NEXT_PUBLIC_TRACKER_URL || "http://localhost:8085";

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
}: {
  site: Site | null;
  token: string;
  loadError: string;
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

  const installCode = `<script async src="${trackerUrl}/track.js" data-site="${site.site_key}"></script>`;

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
    </div>
  );
}