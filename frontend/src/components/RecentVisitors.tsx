"use client";

import { useEffect, useState } from "react";
import { apiFetch } from "@/lib/auth";
import type { Visitor } from "@/lib/types";
import { COUNTRY_NAMES } from "./countryPaths";
import { IconGlobe } from "@/components/icons";

function flagEmoji(cc: string) {
  if (!cc || cc === "unknown" || cc.length !== 2) return "🌐";
  return cc
    .toUpperCase()
    .replace(/./g, (c) => String.fromCodePoint(127397 + c.charCodeAt(0)));
}

const nf = new Intl.NumberFormat("en-US");

export default function RecentVisitors({
  token,
  siteId,
}: {
  token: string;
  siteId: string;
}) {
  const [rows, setRows] = useState<Visitor[]>([]);
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    async function load() {
      try {
        const r = await apiFetch<Visitor[]>(`/api/sites/${siteId}/visitors`, token);
        if (active) {
          setRows(r);
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
  }, [token, siteId]);

  return (
    <section className="rounded-xl border border-edge bg-card p-5">
      <h2 className="mb-4 flex items-center gap-2 text-sm font-semibold text-ink">
        <IconGlobe className="h-4 w-4 text-indigo-500" />
        Recent visitors
      </h2>
      {error && <p className="py-1 text-xs text-red-500">{error}</p>}
      {!error && rows.length === 0 && (
        <p className="py-1 text-sm text-faint">No visitor IPs recorded yet.</p>
      )}
      {rows.length > 0 && (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <tbody>
              {rows.map((r, i) => (
                <tr key={i} className="border-b border-edge last:border-0">
                  <td className="whitespace-nowrap py-2.5 pr-4 font-mono text-xs text-soft">
                    {r.ip}
                  </td>
                  <td className="py-2.5 pr-4 text-xs text-soft">
                    {flagEmoji(r.country)}{" "}
                    <span className="text-faint">{COUNTRY_NAMES[r.country] || r.country}</span>
                  </td>
                  <td className="whitespace-nowrap py-2.5 pr-4 text-xs text-faint">
                    {r.browser} · {r.os} · {r.device}
                  </td>
                  <td className="max-w-40 truncate py-2.5 pr-4 text-xs text-faint" title={r.path}>
                    {r.path}
                  </td>
                  <td className="whitespace-nowrap py-2.5 text-right text-xs tabular-nums text-faint">
                    {new Date(r.visited_at).toLocaleString()}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}