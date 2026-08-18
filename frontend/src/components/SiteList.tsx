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
    if (!confirm("Hapus situs ini beserta seluruh datanya?")) return;
    await apiFetch(`/api/sites/${id}`, token, { method: "DELETE" });
    setSites((s) => s.filter((x) => x.id !== id));
  }

  const inputCls =
    "w-full rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 placeholder-zinc-500 outline-none transition-colors focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500";

  return (
    <div className="mt-8 space-y-8">
      <section id="install">
        <h2 className="text-sm font-semibold text-zinc-300">Cara pasang</h2>
        <ol className="mt-3 grid gap-3 sm:grid-cols-3">
          {[
            ["1", "Buat situs", "Klik “Tambah situs” dan isi nama + domain."],
            ["2", "Salin kode", "Salin satu baris script dari kartu situs."],
            ["3", "Tempel di situs", "Letakkan sebelum </body>. Data langsung masuk."],
          ].map(([n, t, d]) => (
            <li
              key={n}
              className="rounded-xl border border-zinc-800 bg-zinc-900/60 p-4"
            >
              <div className="flex h-6 w-6 items-center justify-center rounded-full bg-indigo-500/15 text-xs font-semibold text-indigo-300">
                {n}
              </div>
              <p className="mt-2.5 text-sm font-medium text-zinc-200">{t}</p>
              <p className="mt-0.5 text-xs leading-relaxed text-zinc-500">{d}</p>
            </li>
          ))}
        </ol>
      </section>

      <section>
        <form
          onSubmit={create}
          className="flex flex-wrap items-end gap-3 rounded-xl border border-zinc-800 bg-zinc-900/60 p-5"
        >
          <div className="w-44">
            <label className="text-xs font-medium text-zinc-400">Nama situs</label>
            <input
              required
              value={name}
              onChange={(e) => setName(e.target.value)}
              className={`${inputCls} mt-1.5`}
              placeholder="Blog saya"
            />
          </div>
          <div className="w-44">
            <label className="text-xs font-medium text-zinc-400">Domain</label>
            <input
              value={domain}
              onChange={(e) => setDomain(e.target.value)}
              className={`${inputCls} mt-1.5`}
              placeholder="contoh.com"
            />
          </div>
          <button
            type="submit"
            disabled={creating}
            className="flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
          >
            <IconPlus className="h-4 w-4" />
            {creating ? "Membuat..." : "Tambah situs"}
          </button>
          {error && <p className="w-full text-sm text-red-400">{error}</p>}
        </form>

        {sites.length === 0 ? (
          <div className="mt-6 flex flex-col items-center rounded-xl border border-dashed border-zinc-800 py-16 text-center">
            <svg
              className="h-16 w-16 text-zinc-700"
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
            <p className="mt-4 text-sm font-medium text-zinc-300">
              Belum ada situs
            </p>
            <p className="mt-1 text-sm text-zinc-500">
              Tambahkan situs pertamamu lewat form di atas.
            </p>
          </div>
        ) : (
          <div className="mt-6 space-y-4">
            {sites.map((site) => (
              <div
                key={site.id}
                className="rounded-xl border border-zinc-800 bg-zinc-900/60 p-5 transition-colors hover:border-zinc-700"
              >
                <div className="flex items-start justify-between gap-4">
                  <div className="flex min-w-0 items-start gap-3.5">
                    <span className="mt-0.5 flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border border-zinc-800 bg-zinc-950 text-indigo-400">
                      <IconGlobe className="h-5 w-5" />
                    </span>
                    <div className="min-w-0">
                      <div className="flex items-center gap-2.5">
                        <Link
                          href={`/sites/${site.id}`}
                          className="font-semibold text-zinc-100 hover:text-indigo-400"
                        >
                          {site.name}
                        </Link>
                        <span className="rounded-full border border-zinc-800 px-2 py-0.5 text-[11px] text-zinc-500">
                          {site.domain || "tanpa domain"}
                        </span>
                      </div>
                      <div className="mt-2 flex flex-wrap items-center gap-2">
                        <code className="truncate rounded-md border border-zinc-800 bg-zinc-950 px-2.5 py-1.5 font-mono text-xs text-zinc-300">
                          {installCode(site)}
                        </code>
                        <button
                          onClick={() => {
                            navigator.clipboard.writeText(installCode(site));
                            setCopied(site.id);
                            setTimeout(() => setCopied(null), 1500);
                          }}
                          className="flex items-center gap-1.5 rounded-md border border-zinc-700 px-2.5 py-1.5 text-xs text-zinc-300 transition-colors hover:bg-zinc-800"
                        >
                          {copied === site.id ? (
                            <IconCheck className="h-3.5 w-3.5 text-emerald-400" />
                          ) : (
                            <IconCopy className="h-3.5 w-3.5" />
                          )}
                          {copied === site.id ? "Tersalin" : "Salin"}
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
                      Statistik
                    </Link>
                    <button
                      onClick={() => remove(site.id)}
                      title="Hapus situs"
                      className="rounded-lg border border-zinc-800 p-2 text-zinc-500 transition-colors hover:border-red-900/60 hover:bg-red-950/40 hover:text-red-400"
                    >
                      <IconTrash className="h-4 w-4" />
                    </button>
                  </div>
                </div>
                <p className="mt-3 flex items-center gap-1.5 text-xs text-zinc-500">
                  <IconCode className="h-3.5 w-3.5" />
                  Site key:{" "}
                  <code className="text-indigo-300">{site.site_key}</code>
                </p>
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}