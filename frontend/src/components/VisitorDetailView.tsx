"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { apiFetch } from "@/lib/auth";
import type { VisitorDetail } from "@/lib/types";
import { COUNTRY_NAMES, COUNTRY_PATHS } from "./countryPaths";
import { IconArrowLeft, IconGlobe, IconShieldCheck } from "@/components/icons";

function flagEmoji(cc: string) {
  if (!cc || cc === "unknown" || cc.length !== 2) return "🌐";
  return cc
    .toUpperCase()
    .replace(/./g, (c) => String.fromCodePoint(127397 + c.charCodeAt(0)));
}

const nf = new Intl.NumberFormat("en-US");

function DetailRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-4 border-b border-edge/60 py-2.5 text-sm last:border-0">
      <span className="text-faint">{label}</span>
      <span className="text-right font-medium text-ink">{value || "—"}</span>
    </div>
  );
}

const W = 720;
const H = 360;
const px = (lng: number) => ((lng + 180) / 360) * W;
const py = (lat: number) => ((90 - lat) / 180) * H;

export default function VisitorDetailView({
  siteId,
  token,
  ip,
  siteName,
}: {
  siteId: string;
  token: string;
  ip: string;
  siteName: string;
}) {
  const router = useRouter();
  const [d, setD] = useState<VisitorDetail | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    async function load() {
      try {
        const r = await apiFetch<VisitorDetail>(
          `/api/sites/${siteId}/visitors/${encodeURIComponent(ip)}`,
          token,
        );
        if (active) {
          setD(r);
          setError("");
        }
      } catch (e: any) {
        if (active) setError(e.message);
      }
    }
    load();
    const t = setInterval(load, 30000);
    return () => {
      active = false;
      clearInterval(t);
    };
  }, [siteId, token, ip]);

  const cc = d?.country_code || "";

  return (
    <div className="mx-auto max-w-5xl px-6 py-8 lg:px-10">
      <button
        onClick={() => router.push(`/sites/${siteId}`)}
        className="mb-6 flex items-center gap-2 text-sm text-faint transition-colors hover:text-ink"
      >
        <IconArrowLeft className="h-4 w-4" /> Back to {siteName}
      </button>

      {error && (
        <div className="rounded-xl border border-red-900/40 bg-red-950/40 p-6 text-sm text-red-400">
          {error}
        </div>
      )}

      {!error && !d && <p className="text-sm text-faint">Loading visitor…</p>}

      {d && (
        <div className="space-y-5">
          <div className="flex items-center gap-3">
            <div className="flex h-11 w-11 items-center justify-center rounded-xl bg-indigo-500/15">
              <IconGlobe className="h-5 w-5 text-indigo-400" />
            </div>
            <div>
              <h1 className="font-mono text-2xl font-semibold text-ink">{d.ip}</h1>
              <p className="text-sm text-faint">
                {flagEmoji(cc)} {COUNTRY_NAMES[cc] || d.country || "Unknown"}
              </p>
            </div>
            <div className="ml-auto rounded-full bg-raised px-3 py-1 text-xs text-faint">
              {nf.format(d.pageviews)} pageview{d.pageviews > 1 ? "s" : ""}
            </div>
          </div>

          <section className="overflow-hidden rounded-xl border border-edge bg-card">
            <div className="flex items-center justify-between px-5 pt-5">
              <h2 className="flex items-center gap-2 text-sm font-semibold text-ink">
                <IconShieldCheck className="h-4 w-4 text-indigo-500" /> Location
              </h2>
              <p className="text-xs text-faint">
                {flagEmoji(cc)} {[d.city, d.region, COUNTRY_NAMES[cc] || d.country].filter(Boolean).join(", ") || "Unknown"}
              </p>
            </div>
            {d.lat !== 0 && d.lon !== 0 ? (
              <svg viewBox={`0 0 ${W} ${H}`} className="mt-3 w-full" preserveAspectRatio="xMidYMid meet">
                <rect x="0" y="0" width={W} height={H} fill="var(--raised)" />
                <path
                  d={COUNTRY_PATHS[cc] || ""}
                  fill="currentColor"
                  className="text-edge opacity-60"
                />
                <circle cx={px(d.lon)} cy={py(d.lat)} r={9} fill="#6366f1" opacity={0.95} />
                <circle cx={px(d.lon)} cy={py(d.lat)} r={22} fill="#6366f1" opacity={0.2} />
                <text
                  x={Math.min(Math.max(px(d.lon) + 14, 6), W - 70)}
                  y={Math.max(py(d.lat) - 10, 14)}
                  className="fill-ink"
                  fontSize="13"
                  fontWeight="700"
                >
                  {d.city || d.region || d.country}
                </text>
              </svg>
            ) : (
              <p className="px-5 pb-5 pt-2 text-sm text-faint">Location unknown.</p>
            )}
          </section>

          <div className="grid gap-5 md:grid-cols-2">
            <section className="rounded-xl border border-edge bg-card p-5">
              <h2 className="mb-3 text-sm font-semibold text-ink">Details</h2>
              <DetailRow label="ISP" value={d.isp === "unknown" ? "—" : d.isp} />
              <DetailRow label="Country" value={COUNTRY_NAMES[cc] || d.country} />
              <DetailRow label="Region" value={d.region} />
              <DetailRow label="City" value={d.city} />
              <DetailRow label="Browser" value={d.browser} />
              <DetailRow label="Operating system" value={d.os} />
              <DetailRow label="Device" value={d.device} />
              <DetailRow label="Screen" value={d.screen} />
              <DetailRow label="Language" value={d.lang} />
              <DetailRow label="First seen" value={new Date(d.first_seen).toLocaleString()} />
              <DetailRow label="Last seen" value={new Date(d.last_seen).toLocaleString()} />
              <DetailRow label="Sessions" value={String(d.sessions)} />
            </section>

            <section className="rounded-xl border border-edge bg-card p-5">
              <h2 className="mb-3 text-sm font-semibold text-ink">Pages visited</h2>
              {d.paths.length === 0 && <p className="text-sm text-faint">No pages recorded.</p>}
              <ul className="space-y-2">
                {d.paths.map((p) => (
                  <li key={p.key} className="flex items-center gap-3 text-sm">
                    <span className="min-w-0 flex-1 truncate font-mono text-xs text-soft">{p.key}</span>
                    <span className="h-1 w-28 overflow-hidden rounded-full bg-raised">
                      <span
                        className="block h-full rounded-full bg-indigo-500/80"
                        style={{ width: `${(p.value / (d.paths[0]?.value || 1)) * 100}%` }}
                      />
                    </span>
                    <span className="w-8 text-right tabular-nums text-faint">{p.value}</span>
                  </li>
                ))}
              </ul>
            </section>
          </div>

          {d.history.length > 0 && (
            <section className="rounded-xl border border-edge bg-card p-5">
              <h2 className="mb-3 text-sm font-semibold text-ink">History</h2>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <tbody>
                    {d.history.map((h, i) => (
                      <tr key={i} className="border-b border-edge last:border-0">
                        <td className="max-w-40 truncate py-2 pr-4 font-mono text-xs text-soft">{h.path}</td>
                        <td className="whitespace-nowrap py-2 pr-4 text-xs text-faint">
                          {flagEmoji(h.country)} {COUNTRY_NAMES[h.country] || h.country}
                        </td>
                        <td className="whitespace-nowrap py-2 pr-4 text-xs text-faint">{h.browser} · {h.os}</td>
                        <td className="whitespace-nowrap py-2 text-right text-xs tabular-nums text-faint">
                          {new Date(h.visited_at).toLocaleString()}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>
          )}

          <p className="text-xs text-faint">
            <Link href={`/sites/${siteId}`} className="text-indigo-400 hover:underline">
              Back to site stats
            </Link>
          </p>
        </div>
      )}
    </div>
  );
}