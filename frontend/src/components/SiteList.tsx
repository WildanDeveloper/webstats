"use client";

import { useState } from "react";
import Link from "next/link";
import { apiFetch } from "@/lib/auth";
import type { Site } from "@/lib/types";
import {
  IconGlobe,
  IconPlus,
  IconTrash,
  IconCopy,
  IconCheck,
  IconChart,
  IconCode,
  IconSettings,
  IconShieldCheck,
  IconShieldX,
} from "@/components/icons";

const trackerUrl =
  process.env.NEXT_PUBLIC_TRACKER_URL || "http://localhost:8085";

export default function SiteList({
  initial,
  token,
}: {
  initial: Site[];
  token: string;
}) {
  const [sites, setSites] = useState<Site[]>(initial);
  const [name, setName] = useState("");
  const [domain, setDomain] = useState("");
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState("");
  const [copied, setCopied] = useState<string | null>(null);

  const installCode = (site: Site) =>
    `<script async src="${trackerUrl}/track.js" data-site="${site.site_key}"></script>`;

  async function create(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setCreating(true);
    try {
      const site = await apiFetch<Site>("/api/sites", token, {
        method: "POST",
        body: JSON.stringify({ name, domain }),
      });
      setSites((s) => [site, ...s]);
      setName("");
      setDomain("");
    } catch (err: any) {
      setError(err.message);
    }
    setCreating(false);
  }

  async function remove(id: string) {
    if (!confirm("Delete this site and all of its data?")) return;
    await apiFetch(`/api/sites/${id}`, token, { method: "DELETE" });
    setSites((s) => s.filter((x) => x.id !== id));
  }

  const inputCls =
    "w-full rounded-lg border border-edge bg-bg px-3 py-2 text-sm text-ink placeholder-faint outline-none transition-colors focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500";

  return (
    <div className="mt-8 space-y-8">
      <section id="install">
        <h2 className="text-sm font-semibold text-ink">How to install</h2>
        <ol className="mt-3 grid gap-3 sm:grid-cols-3">
          {[
            ["1", "Create a site", 'Click "Add site" and enter a name and domain.'],
            ["2", "Copy the script", "Copy the one-line script from the site card."],
            ["3", "Paste on your site", "Place it before </body>. Data starts flowing."],
          ].map(([n, t, d]) => (
            <li key={n} className="rounded-xl border border-edge bg-card p-4">
              <div className="flex h-6 w-6 items-center justify-center rounded-full bg-indigo-500/15 text-xs font-semibold text-indigo-500">
                {n}
              </div>
              <p className="mt-2.5 text-sm font-medium text-ink">{t}</p>
              <p className="mt-0.5 text-xs leading-relaxed text-faint">{d}</p>
            </li>
          ))}
        </ol>
      </section>

      <section>
        <form
          onSubmit={create}
          className="flex flex-wrap items-end gap-3 rounded-xl border border-edge bg-card p-5"
        >
          <div className="w-44">
            <label className="text-xs font-medium text-soft">Site name</label>
            <input
              required
              value={name}
              onChange={(e) => setName(e.target.value)}
              className={`${inputCls} mt-1.5`}
              placeholder="My blog"
            />
          </div>
          <div className="w-44">
            <label className="text-xs font-medium text-soft">Domain</label>
            <input
              value={domain}
              onChange={(e) => setDomain(e.target.value)}
              className={`${inputCls} mt-1.5`}
              placeholder="example.com"
            />
          </div>
          <button
            type="submit"
            disabled={creating}
            className="flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
          >
            <IconPlus className="h-4 w-4" />
            {creating ? "Creating..." : "Add site"}
          </button>
          {error && <p className="w-full text-sm text-red-400">{error}</p>}
        </form>

        {sites.length === 0 ? (
          <div className="mt-6 flex flex-col items-center rounded-xl border border-dashed border-edge py-16 text-center">
            <svg
              className="h-16 w-16 text-faint"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.2"
              strokeLinecap="round"
            >
              <rect x="3" y="3" width="18" height="14" rx="2" />
              <path d="M3 9h18" strokeDasharray="3 3" />
              <circle cx="9" cy="6" r="1" fill="currentColor" />
              <circle cx="13" cy="6" r="1" fill="currentColor" />
              <path d="M3 21h18" />
            </svg>
            <p className="mt-4 text-sm font-medium text-ink">No sites yet</p>
            <p className="mt-1 text-sm text-faint">
              Add your first website using the form above.
            </p>
          </div>
        ) : (
          <div className="mt-6 space-y-4">
            {sites.map((site) => (
              <div
                key={site.id}
                className="rounded-xl border border-edge bg-card p-5 transition-colors hover:border-faint"
              >
                <div className="flex items-start justify-between gap-4">
                  <div className="flex min-w-0 items-start gap-3.5">
                    <span
                      className="mt-1 h-3 w-3 shrink-0 rounded-full"
                      style={{ background: site.color || "#6366f1" }}
                    />
                    <div className="min-w-0">
                      <div className="flex items-center gap-2.5">
                        <Link
                          href={`/sites/${site.id}`}
                          className="font-semibold text-ink hover:text-indigo-500"
                        >
                          {site.name}
                        </Link>
                        <span className="rounded-full border border-edge px-2 py-0.5 text-[11px] text-faint">
                          {site.domain || "no domain"}
                        </span>
                        {site.status && (
                          <span
                            className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium ${
                              site.status === "up"
                                ? "bg-emerald-500/10 text-emerald-500"
                                : "bg-red-950/60 text-red-400"
                            }`}
                          >
                            {site.status === "up" ? (
                              <IconShieldCheck className="h-3 w-3" />
                            ) : (
                              <IconShieldX className="h-3 w-3" />
                            )}
                            {site.status === "up" ? "Online" : "Offline"}
                          </span>
                        )}
                      </div>
                      <div className="mt-2 flex flex-wrap items-center gap-2">
                        <code className="truncate rounded-md border border-edge bg-raised px-2.5 py-1.5 font-mono text-xs text-soft">
                          {installCode(site)}
                        </code>
                        <button
                          onClick={() => {
                            navigator.clipboard.writeText(installCode(site));
                            setCopied(site.id);
                            setTimeout(() => setCopied(null), 1500);
                          }}
                          className="flex items-center gap-1.5 rounded-md border border-edge px-2.5 py-1.5 text-xs text-soft transition-colors hover:bg-raised"
                        >
                          {copied === site.id ? (
                            <IconCheck className="h-3.5 w-3.5 text-emerald-500" />
                          ) : (
                            <IconCopy className="h-3.5 w-3.5" />
                          )}
                          {copied === site.id ? "Copied" : "Copy"}
                        </button>
                      </div>
                    </div>
                  </div>
                  <div className="flex shrink-0 items-center gap-2">
                    <Link
                      href={`/sites/${site.id}`}
                      className="flex items-center gap-2 rounded-lg bg-indigo-600 px-3.5 py-2 text-sm font-medium text-white transition-colors hover:bg-indigo-500"
                    >
                      <IconChart className="h-4 w-4" />
                      Stats
                    </Link>
                    <Link
                      href={`/sites/${site.id}/settings`}
                      title="Site settings"
                      className="rounded-lg border border-edge p-2 text-faint transition-colors hover:bg-raised hover:text-ink"
                    >
                      <IconSettings className="h-4 w-4" />
                    </Link>
                    <button
                      onClick={() => remove(site.id)}
                      title="Delete site"
                      className="rounded-lg border border-edge p-2 text-faint transition-colors hover:border-red-900/60 hover:bg-red-950/40 hover:text-red-400"
                    >
                      <IconTrash className="h-4 w-4" />
                    </button>
                  </div>
                </div>
                <p className="mt-3 flex items-center gap-1.5 text-xs text-faint">
                  <IconCode className="h-3.5 w-3.5" />
                  Site key: <code className="text-indigo-500">{site.site_key}</code>
                </p>
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}