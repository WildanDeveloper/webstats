"use client";

import { useState } from "react";
import { useRouter, usePathname } from "next/navigation";
import Link from "next/link";
import type { Overview, TimePoint, Row, EventRow, Site, WorldPoint } from "@/lib/types";
import StatCards from "@/components/charts/StatCards";
import VisitorsChart from "@/components/charts/VisitorsChart";
import TopList from "@/components/charts/TopList";
import DonutChart from "@/components/charts/DonutChart";
import RealtimePanel from "@/components/RealtimePanel";
import WorldMap from "@/components/WorldMap";
import {
  IconArrowLeft,
  IconGlobe,
  IconPulse,
  IconSettings,
  IconDownload,
  IconShieldCheck,
  IconShieldX,
} from "@/components/icons";

const PERIODS = [
  { key: "24h", label: "24h" },
  { key: "7d", label: "7 days" },
  { key: "30d", label: "30 days" },
  { key: "all", label: "All" },
];

function Card({
  title,
  icon,
  children,
}: {
  title: string;
  icon?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <section className="rounded-xl border border-edge bg-card p-5">
      <h2 className="mb-4 flex items-center gap-2 text-sm font-semibold text-ink">
        {icon}
        {title}
      </h2>
      {children}
    </section>
  );
}

function StatusBadge({ site }: { site: Site | null }) {
  if (!site?.status) return null;
  const up = site.status === "up";
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[11px] font-medium ${
        up ? "bg-emerald-500/10 text-emerald-500" : "bg-red-950/60 text-red-400"
      }`}
      title={site.checked_at ? `Last check: ${new Date(site.checked_at).toLocaleString()}` : ""}
    >
      {up ? (
        <IconShieldCheck className="h-3 w-3" />
      ) : (
        <IconShieldX className="h-3 w-3" />
      )}
      {up ? "Online" : "Offline"}
      {site.latency_ms ? ` · ${site.latency_ms}ms` : ""}
    </span>
  );
}

export default function StatsView(props: {
  site: Site | null;
  siteId: string;
  token: string;
  period: string;
  from: string;
  to: string;
  overview: Overview | null;
  timeseries: TimePoint[];
  pages: Row[];
  referrers: Row[];
  devices: Row[];
  browsers: Row[];
  os: Row[];
  countries: Row[];
  events: EventRow[];
  world: WorldPoint[];
  error: string;
}) {
  const router = useRouter();
  const pathname = usePathname();
  const { siteId, token, period, from, to } = props;
  const [fromD, setFromD] = useState(from);
  const [toD, setToD] = useState(to);

  function applyRange() {
    if (!fromD || !toD) return;
    router.push(`${pathname}?from=${fromD}&to=${toD}`);
  }

  async function exportCsv() {
    try {
      const res = await fetch(
        `${process.env.NEXT_PUBLIC_API_URL}/api/sites/${siteId}/export?${from && to ? `from=${from}&to=${to}` : `period=${period}`}`,
        { headers: { Authorization: `Bearer ${token}` } },
      );
      if (!res.ok) throw new Error("export failed");
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `webstats-${props.site?.name || "site"}-${from && to ? `${from}_${to}` : period}.csv`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (e: any) {
      alert(e.message || "Export failed");
    }
  }

  return (
    <div className="space-y-6">
      <header>
        <div className="flex items-center justify-between">
          <Link
            href="/"
            className="inline-flex items-center gap-1.5 text-sm text-faint transition-colors hover:text-ink"
          >
            <IconArrowLeft className="h-3.5 w-3.5" />
            Dashboard
          </Link>
          <Link
            href={`/sites/${props.site?.id}/settings`}
            className="inline-flex items-center gap-1.5 text-sm text-faint transition-colors hover:text-ink"
          >
            <IconSettings className="h-3.5 w-3.5" />
            Settings
          </Link>
        </div>
        <div className="mt-1 flex flex-wrap items-center justify-between gap-4">
          <div>
            <h1 className="flex items-center gap-2.5 text-2xl font-semibold tracking-tight text-ink">
              <span
                className="h-3 w-3 rounded-full"
                style={{ background: props.site?.color || "#6366f1" }}
              />
              {props.site?.name || "Site"}
              <StatusBadge site={props.site} />
            </h1>
            <p className="mt-0.5 flex items-center gap-1.5 text-sm text-faint">
              <IconGlobe className="h-3.5 w-3.5" />
              {props.site?.domain || "no domain"}
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <div className="flex items-center gap-1.5">
              <input
                type="date"
                value={fromD}
                onChange={(e) => setFromD(e.target.value)}
                className="rounded-lg border border-edge bg-card px-2.5 py-1.5 text-xs text-ink outline-none focus:border-indigo-500"
              />
              <span className="text-xs text-faint">to</span>
              <input
                type="date"
                value={toD}
                onChange={(e) => setToD(e.target.value)}
                className="rounded-lg border border-edge bg-card px-2.5 py-1.5 text-xs text-ink outline-none focus:border-indigo-500"
              />
              <button
                onClick={applyRange}
                disabled={!fromD || !toD}
                className="rounded-lg bg-raised px-2.5 py-1.5 text-xs font-medium text-ink transition-colors hover:bg-soft/40 disabled:opacity-50"
              >
                Apply
              </button>
            </div>
            <button
              onClick={exportCsv}
              title="Export CSV"
              className="inline-flex items-center gap-1.5 rounded-lg border border-edge bg-card px-3 py-1.5 text-xs font-medium text-soft transition-colors hover:text-ink"
            >
              <IconDownload className="h-3.5 w-3.5" />
              CSV
            </button>
            <div className="flex rounded-lg border border-edge bg-card p-1">
              {PERIODS.map((p) => (
                <button
                  key={p.key}
                  onClick={() => router.push(`${pathname}?period=${p.key}`)}
                  className={`rounded-md px-3.5 py-1.5 text-sm transition-colors ${
                    period === p.key && !from && !to
                      ? "bg-raised font-medium text-ink"
                      : "text-faint hover:text-ink"
                  }`}
                >
                  {p.label}
                </button>
              ))}
            </div>
          </div>
        </div>
      </header>

      {props.error ? (
        <div className="rounded-xl border border-red-900/60 bg-red-950/40 p-6 text-sm text-red-300">
          Failed to load data: {props.error}
        </div>
      ) : (
        <>
          <RealtimePanel token={token} siteId={siteId} />

          <StatCards
            pageviews={props.overview?.pageviews ?? 0}
            visitors={props.overview?.visitors ?? 0}
            bounceRate={props.overview?.bounce_rate ?? 0}
            avgPerDay={props.overview?.avg_per_day ?? 0}
            prevPageviews={props.overview?.prev_pageviews ?? 0}
            prevVisitors={props.overview?.prev_visitors ?? 0}
          />

          <Card
            title="Visitor trend"
            icon={<IconPulse className="h-4 w-4 text-indigo-500" />}
          >
            <VisitorsChart data={props.timeseries} />
          </Card>

          <div className="grid gap-6 lg:grid-cols-2">
            <TopList title="Top pages" rows={props.pages} />
            <TopList title="Referrers" rows={props.referrers} />
            <DonutChart title="Devices" rows={props.devices} />
            <DonutChart title="Browsers" rows={props.browsers} />
            <DonutChart title="Operating systems" rows={props.os} />
            <DonutChart title="Countries" rows={props.countries} />
          </div>

          <WorldMap points={props.world} />

          {props.events.length > 0 && (
            <TopList
              title="Custom events"
              rows={props.events.map((e) => ({ key: e.name, value: e.count }))}
            />
          )}
        </>
      )}
    </div>
  );
}