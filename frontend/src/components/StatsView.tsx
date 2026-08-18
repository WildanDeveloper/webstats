"use client";

import { useState } from "react";
import { useRouter, usePathname } from "next/navigation";
import Link from "next/link";
import { apiFetch } from "@/lib/auth";
import type {
  Campaign,
  EventRow,
  FunnelStep,
  Goal,
  GoalSummary,
  Overview,
  Row,
  Site,
  TimePoint,
  WorldPoint,
} from "@/lib/types";
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
  IconPlus,
  IconTrash,
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
  right,
  children,
}: {
  title: string;
  icon?: React.ReactNode;
  right?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <section className="rounded-xl border border-edge bg-card p-5">
      <div className="mb-4 flex items-center justify-between">
        <h2 className="flex items-center gap-2 text-sm font-semibold text-ink">
          {icon}
          {title}
        </h2>
        {right}
      </div>
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
  campaigns: Campaign[];
  goals: GoalSummary[];
  error: string;
}) {
  const router = useRouter();
  const pathname = usePathname();
  const { siteId, token, period, from, to } = props;
  const [fromD, setFromD] = useState(from);
  const [toD, setToD] = useState(to);
  const [goals, setGoals] = useState(props.goals);
  const [goalName, setGoalName] = useState("");
  const [goalPath, setGoalPath] = useState("");
  const [goalMatch, setGoalMatch] = useState("contains");
  const [funnelInput, setFunnelInput] = useState("");
  const [funnel, setFunnel] = useState<FunnelStep[]>([]);
  const [goalMsg, setGoalMsg] = useState("");

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

  async function addGoal(e: React.FormEvent) {
    e.preventDefault();
    setGoalMsg("");
    try {
      const g = await apiFetch<Goal>(`/api/sites/${siteId}/goals`, token, {
        method: "POST",
        body: JSON.stringify({ name: goalName, path: goalPath, match_type: goalMatch }),
      });
      setGoals((prev) => [...prev, { ...g, pageviews: 0, conversions: 0, conversion_pct: 0 }]);
      setGoalName("");
      setGoalPath("");
    } catch (err: any) {
      setGoalMsg(err.message);
    }
  }

  async function deleteGoal(id: string) {
    setGoalMsg("");
    try {
      await apiFetch(`/api/sites/${siteId}/goals/${id}`, token, { method: "DELETE" });
      setGoals((prev) => prev.filter((g) => g.id !== id));
    } catch (err: any) {
      setGoalMsg(err.message);
    }
  }

  async function runFunnel(e: React.FormEvent) {
    e.preventDefault();
    setGoalMsg("");
    const paths = funnelInput.split(",").map((p) => p.trim()).filter(Boolean);
    if (paths.length < 2) {
      setGoalMsg("Enter at least 2 paths separated by commas");
      return;
    }
    try {
      const res = await apiFetch<{ steps: FunnelStep[] }>(`/api/sites/${siteId}/funnel?${from && to ? `from=${from}&to=${to}` : `period=${period}`}`, token, {
        method: "POST",
        body: JSON.stringify({ paths }),
      });
      setFunnel(res.steps);
    } catch (err: any) {
      setGoalMsg(err.message);
    }
  }

  const maxFunnel = funnel.length ? Math.max(...funnel.map((f) => f.sessions)) : 0;

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

          {props.campaigns.length > 0 && (
            <Card title="Campaigns (UTM)" icon={<IconGlobe className="h-4 w-4 text-indigo-500" />}>
              <div className="overflow-x-auto">
                <table className="w-full text-left text-sm">
                  <thead>
                    <tr className="border-b border-edge text-xs uppercase tracking-wide text-faint">
                      <th className="px-3 py-2 font-medium">Source</th>
                      <th className="px-3 py-2 font-medium">Medium</th>
                      <th className="px-3 py-2 font-medium">Campaign</th>
                      <th className="px-3 py-2 font-medium">Content</th>
                      <th className="px-3 py-2 text-right font-medium">Visitors</th>
                      <th className="px-3 py-2 text-right font-medium">Pageviews</th>
                    </tr>
                  </thead>
                  <tbody>
                    {props.campaigns.map((c, i) => (
                      <tr key={i} className="border-b border-edge/60 last:border-0">
                        <td className="px-3 py-2 text-ink">{c.source}</td>
                        <td className="px-3 py-2 text-soft">{c.medium}</td>
                        <td className="px-3 py-2 text-soft">{c.campaign}</td>
                        <td className="px-3 py-2 text-faint">{c.content}</td>
                        <td className="px-3 py-2 text-right text-ink">{c.visitors}</td>
                        <td className="px-3 py-2 text-right text-soft">{c.count}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </Card>
          )}

          <Card
            title="Goals & funnel"
            icon={<IconPulse className="h-4 w-4 text-indigo-500" />}
          >
            <div className="space-y-3">
              <form onSubmit={addGoal} className="flex flex-wrap items-center gap-2">
                <input
                  className="w-40 rounded-lg border border-edge bg-bg px-3 py-1.5 text-sm text-ink outline-none focus:border-indigo-500"
                  placeholder="Goal name"
                  value={goalName}
                  onChange={(e) => setGoalName(e.target.value)}
                  required
                />
                <input
                  className="w-44 rounded-lg border border-edge bg-bg px-3 py-1.5 text-sm text-ink outline-none focus:border-indigo-500"
                  placeholder="/thank-you"
                  value={goalPath}
                  onChange={(e) => setGoalPath(e.target.value)}
                  required
                />
                <select
                  className="rounded-lg border border-edge bg-bg px-2 py-1.5 text-sm text-ink outline-none focus:border-indigo-500"
                  value={goalMatch}
                  onChange={(e) => setGoalMatch(e.target.value)}
                >
                  <option value="contains">contains</option>
                  <option value="exact">exact</option>
                </select>
                <button className="rounded-lg bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-indigo-500">
                  Add goal
                </button>
              </form>
              {goalMsg && <p className="text-xs text-red-500">{goalMsg}</p>}
              <div className="space-y-2">
                {goals.map((g) => (
                    <div key={g.id} className="flex items-center gap-3 rounded-lg border border-edge/60 px-3 py-2">
                      <div className="min-w-0 flex-1">
                        <p className="text-sm font-medium text-ink">
                          {g.name}
                          <span className="ml-2 text-xs font-normal text-faint">{g.path} ({g.match_type})</span>
                        </p>
                        <div className="mt-1 flex items-center gap-2">
                          <div className="h-1.5 w-40 overflow-hidden rounded-full bg-raised">
                            <div
                              className="h-full rounded-full bg-indigo-500"
                              style={{ width: `${Math.min(100, g.conversion_pct)}%` }}
                            />
                          </div>
                          <span className="text-xs text-faint">
                            {g.conversions} conversions · {g.conversion_pct.toFixed(1)}%
                          </span>
                        </div>
                      </div>
                      <button
                        onClick={() => deleteGoal(g.id)}
                        className="rounded-lg p-1.5 text-faint transition-colors hover:bg-raised hover:text-red-500"
                      >
                        <IconTrash className="h-3.5 w-3.5" />
                      </button>
                    </div>
                  ))}
                {goals.length === 0 && (
                  <p className="text-xs text-faint">No goals yet. Add one above.</p>
                )}
              </div>
              <form onSubmit={runFunnel} className="flex flex-wrap items-center gap-2 border-t border-edge pt-3">
                <input
                  className="min-w-56 flex-1 rounded-lg border border-edge bg-bg px-3 py-1.5 text-sm text-ink outline-none focus:border-indigo-500"
                  placeholder="Funnel paths, e.g. /landing, /pricing, /thank-you"
                  value={funnelInput}
                  onChange={(e) => setFunnelInput(e.target.value)}
                />
                <button className="rounded-lg bg-raised px-3 py-1.5 text-sm font-medium text-ink hover:bg-soft/40">
                  Analyze
                </button>
              </form>
              {funnel.length > 0 && (
                <div className="space-y-1.5">
                  {funnel.map((f, i) => (
                    <div key={i} className="flex items-center gap-2">
                      <span className="w-32 truncate text-xs text-soft" title={f.path}>{f.path}</span>
                      <div className="h-4 flex-1 overflow-hidden rounded bg-raised">
                        <div
                          className={`h-full ${f.label === "converted" ? "bg-emerald-500" : "bg-indigo-500"}`}
                          style={{ width: `${maxFunnel ? (f.sessions / maxFunnel) * 100 : 0}%` }}
                        />
                      </div>
                      <span className="w-8 text-right text-xs font-medium text-ink">{f.sessions}</span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </Card>

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