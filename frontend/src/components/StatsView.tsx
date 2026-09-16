"use client";

import { useEffect, useState } from "react";
import { useRouter, usePathname } from "next/navigation";
import Link from "next/link";
import { apiFetch, CLIENT_API_URL } from "@/lib/auth";
import type {
  Campaign,
  EventDetail,
  EventOccurrence,
  FunnelStep,
  Goal,
  GoalSummary,
  Insights,
  Overview,
  Row,
  Site,
  TimePoint,
  Vitals,
  WorldPoint,
} from "@/lib/types";
import StatCards from "@/components/charts/StatCards";
import VisitorsChart from "@/components/charts/VisitorsChart";
import TopList from "@/components/charts/TopList";
import DonutChart from "@/components/charts/DonutChart";
import RealtimePanel from "@/components/RealtimePanel";
import RecentVisitors from "@/components/RecentVisitors";
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
  IconSparkles,
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
  publicToken?: string;
  period: string;
  from: string;
  to: string;
  filters: { page?: string; source?: string; country?: string; device?: string; browser?: string; os?: string };
  overview: Overview | null;
  timeseries: TimePoint[];
  pages: Row[];
  referrers: Row[];
  devices: Row[];
  browsers: Row[];
  os: Row[];
  countries: Row[];
  events: EventDetail[];
  world: WorldPoint[];
  campaigns: Campaign[];
  goals: GoalSummary[];
  insights: Insights | null;
  funnelReport: FunnelStep[];
  vitals?: Vitals | null;
  error: string;
}) {
  const router = useRouter();
  const pathname = usePathname();
  const { siteId, token, period, from, to, filters, publicToken = "" } = props;
  const isPublic = publicToken.length > 0;
  const basePath = isPublic ? `/api/public/${publicToken}` : `/api/sites/${siteId}`;
  const [fromD, setFromD] = useState(from);
  const [toD, setToD] = useState(to);
  const [goals, setGoals] = useState(props.goals);
  const [goalName, setGoalName] = useState("");
  const [goalPath, setGoalPath] = useState("");
  const [goalMatch, setGoalMatch] = useState("contains");
  const [goalMsg, setGoalMsg] = useState("");
  const [openEvent, setOpenEvent] = useState("");
  const [occurrences, setOccurrences] = useState<EventOccurrence[]>([]);
  const [evMsg, setEvMsg] = useState("");
  const [funnel, setFunnel] = useState<FunnelStep[]>(props.funnelReport || []);

  async function pubGet<T>(path: string, qs = "") {
    if (isPublic) {
      const res = await fetch(`${CLIENT_API_URL}${basePath}${path}${qs}`, {
        cache: "no-store",
      });
      if (!res.ok) throw new Error("request failed");
      return (await res.json()) as T;
    }
    return apiFetch<T>(`${basePath}${path}${qs}`, token);
  }

  function baseQ() {
    const parts: string[] = [];
    if (from && to) {
      parts.push(`from=${from}`, `to=${to}`);
    } else {
      parts.push(`period=${period}`);
    }
    return parts.join("&");
  }

  function go(patch: Record<string, string>) {
    const qs = new URLSearchParams();
    if (from && to) {
      qs.set("from", from);
      qs.set("to", to);
    } else {
      qs.set("period", period);
    }
    for (const [k, v] of Object.entries(patch)) {
      if (v) qs.set(k, v);
      else qs.delete(k);
    }
    router.push(`${pathname}?${qs.toString()}`);
  }

  function setFilter(key: string, value: string) {
    const next = { ...filters };
    if (next[key as keyof typeof next] === value) {
      delete next[key as keyof typeof next];
    } else {
      (next as Record<string, string>)[key] = value;
    }
    go(next as Record<string, string>);
  }

  function clearFilters() {
    go({});
  }

  function setPeriod(key: string) {
    const qs = new URLSearchParams();
    qs.set("period", key);
    for (const [k, v] of Object.entries(filters)) {
      if (v) qs.set(k, v);
    }
    router.push(`${pathname}?${qs.toString()}`);
  }

  const filterLabels: [string, string][] = [
    ["page", "Page"],
    ["source", "Source"],
    ["country", "Country"],
    ["device", "Device"],
    ["browser", "Browser"],
    ["os", "OS"],
  ];
  const activeFilters = filterLabels.filter(([k]) => filters[k as keyof typeof filters]);

  async function toggleEvent(name: string) {
    setEvMsg("");
    if (openEvent === name) {
      setOpenEvent("");
      setOccurrences([]);
      return;
    }
    setOpenEvent(name);
    setOccurrences([]);
    try {
      const res = await pubGet<EventOccurrence[]>(
        `/events/${encodeURIComponent(name)}?${baseQ()}`,
      );
      setOccurrences(res);
    } catch (err: any) {
      setEvMsg(err.message);
    }
  }

  function applyRange() {
    if (!fromD || !toD) return;
    go({ from: fromD, to: toD });
  }

  function exportCsv() {
    const qs = new URLSearchParams();
    if (from && to) {
      qs.set("from", from);
      qs.set("to", to);
    } else {
      qs.set("period", period);
    }
    for (const [k, v] of Object.entries(filters)) {
      if (v) qs.set(k, v);
    }
    fetch(`${CLIENT_API_URL}/api/sites/${siteId}/export?${qs.toString()}`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then(async (res) => {
        if (!res.ok) throw new Error("export failed");
        const blob = await res.blob();
        const url = URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = `webstats-${props.site?.name || "site"}-${from && to ? `${from}_${to}` : period}.csv`;
        a.click();
        URL.revokeObjectURL(url);
      })
      .catch((e: any) => alert(e.message || "Export failed"));
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
          {!isPublic && (
            <Link
              href={`/sites/${props.site?.id}/settings`}
              className="inline-flex items-center gap-1.5 text-sm text-faint transition-colors hover:text-ink"
            >
              <IconSettings className="h-3.5 w-3.5" />
              Settings
            </Link>
          )}
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
            {!isPublic && (
              <button
                onClick={exportCsv}
                title="Export CSV"
                className="inline-flex items-center gap-1.5 rounded-lg border border-edge bg-card px-3 py-1.5 text-xs font-medium text-soft transition-colors hover:text-ink"
              >
                <IconDownload className="h-3.5 w-3.5" />
                CSV
              </button>
            )}
            <div className="flex rounded-lg border border-edge bg-card p-1">
              {PERIODS.map((p) => (
                <button
                  key={p.key}
                  onClick={() => setPeriod(p.key)}
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
          {activeFilters.length > 0 && (
            <div className="flex flex-wrap items-center gap-2 rounded-xl border border-edge bg-card px-4 py-3">
              <span className="text-xs font-medium uppercase tracking-wide text-faint">Filters</span>
              {activeFilters.map(([k, label]) => (
                <button
                  key={k}
                  onClick={() => setFilter(k, "")}
                  className="group inline-flex items-center gap-1.5 rounded-full bg-indigo-500/10 px-2.5 py-1 text-xs font-medium text-indigo-400 transition-colors hover:bg-indigo-500/20"
                  title={`Remove ${label} filter`}
                >
                  {label}: {filters[k as keyof typeof filters]}
                  <span className="text-indigo-400/70 group-hover:text-indigo-300">×</span>
                </button>
              ))}
              <button
                onClick={clearFilters}
                className="text-xs font-medium text-faint transition-colors hover:text-ink"
              >
                Clear all
              </button>
            </div>
          )}

          {!isPublic && <RealtimePanel token={token} siteId={siteId} />}

          {!isPublic && <RecentVisitors token={token} siteId={siteId} />}

          {props.insights && (
            <section className="rounded-xl border border-indigo-500/30 bg-indigo-500/5 p-5">
              <h2 className="flex items-center gap-2 text-sm font-semibold text-ink">
                <IconSparkles className="h-4 w-4 text-indigo-400" />
                Insights
              </h2>
              <p className="mt-2 text-sm leading-relaxed text-soft">{props.insights.summary}</p>
              <div className="mt-3 flex flex-wrap gap-2">
                {props.insights.highlights.map((h, i) => (
                  <span key={i} className="rounded-lg border border-edge bg-card px-2.5 py-1.5 text-xs text-soft">
                    <span className="font-medium text-ink">{h.title}:</span> {h.text}
                  </span>
                ))}
              </div>
            </section>
          )}

          <StatCards
            pageviews={props.overview?.pageviews ?? 0}
            visitors={props.overview?.visitors ?? 0}
            bounceRate={props.overview?.bounce_rate ?? 0}
            sessions={props.overview?.sessions ?? 0}
            prevPageviews={props.overview?.prev_pageviews ?? 0}
            prevVisitors={props.overview?.prev_visitors ?? 0}
            prevSessions={props.overview?.prev_sessions ?? 0}
            prevBounceRate={
              props.overview && props.overview.prev_sessions > 0 && props.overview.prev_bounces >= 0
                ? (props.overview.prev_bounces / props.overview.prev_sessions) * 100
                : undefined
            }
          />

          <Card
            title="Visitor trend"
            icon={<IconPulse className="h-4 w-4 text-indigo-500" />}
          >
            <VisitorsChart data={props.timeseries} />
          </Card>

          <div className="grid gap-6 lg:grid-cols-2">
            <TopList title="Top pages" rows={props.pages} onSelect={(k) => setFilter("page", k)} />
            <TopList title="Referrers" rows={props.referrers} onSelect={(k) => setFilter("source", k)} />
            <EngagementCard basePath={basePath} qs={baseQ()} get={pubGet} />
            <EntryExitCard basePath={basePath} qs={baseQ()} get={pubGet} setFilter={setFilter} />
            <DonutChart title="Devices" rows={props.devices} onSelect={(k) => setFilter("device", k)} />
            <DonutChart title="Browsers" rows={props.browsers} onSelect={(k) => setFilter("browser", k)} />
            <DonutChart title="Operating systems" rows={props.os} onSelect={(k) => setFilter("os", k)} />
            <DonutChart title="Countries" rows={props.countries} onSelect={(k) => setFilter("country", k)} flags />
          </div>

          {props.vitals && props.vitals.summary.length > 0 && <WebVitalsCard vitals={props.vitals} />}

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
              {!isPublic && (
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
              )}
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
                      {!isPublic && (
                        <button
                          onClick={() => deleteGoal(g.id)}
                          className="rounded-lg p-1.5 text-faint transition-colors hover:bg-raised hover:text-red-500"
                        >
                          <IconTrash className="h-3.5 w-3.5" />
                        </button>
                      )}
                    </div>
                  ))}
                {goals.length === 0 && (
                  <p className="text-xs text-faint">No goals yet. Add one above.</p>
                )}
              </div>
              {funnel.length > 0 && (
                <div className="mt-3 space-y-1.5">
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
            <Card title="Custom events" icon={<IconPulse className="h-4 w-4 text-indigo-500" />}>
              <div className="space-y-2">
                {props.events.map((e) => (
                  <div key={e.name} className="overflow-hidden rounded-lg border border-edge/60">
                    <button
                      onClick={() => toggleEvent(e.name)}
                      className="flex w-full items-center gap-3 px-3 py-2.5 text-left transition-colors hover:bg-raised"
                    >
                      <span className="min-w-0 flex-1 truncate text-sm font-medium text-ink">{e.name}</span>
                      <span className="text-xs text-faint">
                        {e.visitors} visitors · {e.count} events
                      </span>
                      {e.max_value > 0 && (
                        <span className="text-xs text-faint">
                          avg {e.avg_value.toFixed(1)} · max {e.max_value.toFixed(1)}
                        </span>
                      )}
                      <span className={`text-xs transition-transform ${openEvent === e.name ? "rotate-90" : ""}`}>›</span>
                    </button>
                    {openEvent === e.name && (
                      <div className="border-t border-edge/60 bg-raised/40 px-3 py-2">
                        {evMsg && <p className="py-1 text-xs text-red-500">{evMsg}</p>}
                        {occurrences.length === 0 && !evMsg && (
                          <p className="py-1 text-xs text-faint">No occurrences in this period.</p>
                        )}
                        {occurrences.map((o, i) => (
                          <div key={i} className="flex items-start gap-3 border-b border-edge/40 py-1.5 last:border-0">
                            <span className="whitespace-nowrap text-[11px] text-faint">
                              {new Date(o.created_at).toLocaleString()}
                            </span>
                            <span className="max-w-48 truncate text-xs text-soft" title={o.url}>{o.url || "/"}</span>
                            <span className="min-w-0 flex-1 truncate font-mono text-[11px] text-faint" title={JSON.stringify(o.props)}>
                              {JSON.stringify(o.props)}
                            </span>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            </Card>
          )}
        </>
      )}
    </div>
  );
}

type SessionStatsT = {
  avg_duration_sec: number;
  median_duration_sec: number;
  avg_pages: number;
  bounce_rate: number;
  sessions: number;
  prev_avg_duration_sec: number;
  prev_avg_pages: number;
};

const VITAL_METRICS: { key: string; label: string; good: number; poor: number; unit: string }[] = [
  { key: "lcp", label: "LCP", good: 2500, poor: 4000, unit: "ms" },
  { key: "cls", label: "CLS", good: 0.1, poor: 0.25, unit: "" },
  { key: "inp", label: "INP", good: 200, poor: 500, unit: "ms" },
  { key: "fcp", label: "FCP", good: 1800, poor: 3000, unit: "ms" },
  { key: "ttfb", label: "TTFB", good: 800, poor: 1800, unit: "ms" },
];

function vitalTone(metric: string, v: number) {
  const def = VITAL_METRICS.find((m) => m.key === metric);
  if (!def) return "text-ink";
  if (v <= def.good) return "text-emerald-500";
  if (v <= def.poor) return "text-amber-500";
  return "text-red-400";
}

function fmtVital(metric: string, v: number) {
  const def = VITAL_METRICS.find((m) => m.key === metric);
  if (!def) return v.toFixed(0);
  return def.unit === "ms" ? `${Math.round(v)}ms` : v.toFixed(3);
}

function WebVitalsCard({ vitals }: { vitals: Vitals }) {
  const summary = [...vitals.summary].sort(
    (a, b) =>
      VITAL_METRICS.findIndex((m) => m.key === a.metric) -
      VITAL_METRICS.findIndex((m) => m.key === b.metric),
  );
  return (
    <Card
      title="Web Vitals (p75)"
      icon={<IconPulse className="h-4 w-4 text-indigo-500" />}
      right={<span className="text-[11px] text-faint">opt-in via data-vitals</span>}
    >
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-5">
        {summary.map((s) => (
          <div key={s.metric}>
            <p className="text-xs text-faint">{s.metric.toUpperCase()}</p>
            <p className={`mt-1 text-xl font-semibold ${vitalTone(s.metric, s.p75)}`}>
              {fmtVital(s.metric, s.p75)}
            </p>
            <p className="text-[11px] text-faint">{s.samples} samples</p>
          </div>
        ))}
      </div>
      {vitals.paths.length > 0 && (
        <div className="mt-4 overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead>
              <tr className="border-b border-edge text-xs uppercase tracking-wide text-faint">
                <th className="px-3 py-2 font-medium">Path</th>
                <th className="px-3 py-2 text-right font-medium">LCP</th>
                <th className="px-3 py-2 text-right font-medium">CLS</th>
                <th className="px-3 py-2 text-right font-medium">INP</th>
                <th className="px-3 py-2 text-right font-medium">Samples</th>
              </tr>
            </thead>
            <tbody>
              {vitals.paths.map((p, i) => (
                <tr key={i} className="border-b border-edge/60 last:border-0">
                  <td className="max-w-64 truncate px-3 py-2 text-ink" title={p.path}>{p.path}</td>
                  <td className={`px-3 py-2 text-right ${p.lcp ? vitalTone("lcp", p.lcp) : "text-faint"}`}>{p.lcp ? fmtVital("lcp", p.lcp) : "—"}</td>
                  <td className={`px-3 py-2 text-right ${p.cls ? vitalTone("cls", p.cls) : "text-faint"}`}>{p.cls ? fmtVital("cls", p.cls) : "—"}</td>
                  <td className={`px-3 py-2 text-right ${p.inp ? vitalTone("inp", p.inp) : "text-faint"}`}>{p.inp ? fmtVital("inp", p.inp) : "—"}</td>
                  <td className="px-3 py-2 text-right text-soft">{p.n}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Card>
  );
}

function fmtDuration(sec: number) {
  if (sec <= 0) return "0s";
  const m = Math.floor(sec / 60);
  const s = Math.round(sec % 60);
  return m > 0 ? `${m}m ${s}s` : `${s}s`;
}

function EngagementCard({
  basePath,
  qs,
  get,
}: {
  basePath: string;
  qs: string;
  get: <T>(path: string, qs?: string) => Promise<T>;
}) {
  const [stats, setStats] = useState<SessionStatsT | null>(null);

  useEffect(() => {
    let alive = true;
    get<SessionStatsT>(`/sessions?${qs}`)
      .then((s) => alive && setStats(s))
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, [basePath, qs]);

  const durDelta =
    stats && stats.prev_avg_duration_sec > 0
      ? ((stats.avg_duration_sec - stats.prev_avg_duration_sec) / stats.prev_avg_duration_sec) * 100
      : 0;
  return (
    <Card title="Engagement" icon={<IconPulse className="h-4 w-4 text-indigo-500" />}>
      {stats ? (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <div>
            <p className="text-xs text-faint">Avg. visit</p>
            <p className="mt-1 text-xl font-semibold text-ink">{fmtDuration(stats.avg_duration_sec)}</p>
            <p className={`text-[11px] ${durDelta >= 0 ? "text-emerald-400" : "text-red-400"}`}>
              {durDelta >= 0 ? "+" : ""}
              {durDelta.toFixed(0)}% vs prev
            </p>
          </div>
          <div>
            <p className="text-xs text-faint">Median</p>
            <p className="mt-1 text-xl font-semibold text-ink">{fmtDuration(stats.median_duration_sec)}</p>
          </div>
          <div>
            <p className="text-xs text-faint">Pages / visit</p>
            <p className="mt-1 text-xl font-semibold text-ink">{stats.avg_pages.toFixed(1)}</p>
          </div>
          <div>
            <p className="text-xs text-faint">Bounce rate</p>
            <p className="mt-1 text-xl font-semibold text-ink">{stats.bounce_rate.toFixed(1)}%</p>
          </div>
        </div>
      ) : (
        <p className="text-xs text-faint">No session data in this period.</p>
      )}
    </Card>
  );
}

function EntryExitCard({
  basePath,
  qs,
  get,
  setFilter,
}: {
  basePath: string;
  qs: string;
  get: <T>(path: string, qs?: string) => Promise<T>;
  setFilter: (key: string, value: string) => void;
}) {
  const [tab, setTab] = useState<"entry" | "exit">("entry");
  const [rows, setRows] = useState<Row[]>([]);

  useEffect(() => {
    let alive = true;
    get<Row[]>(`/${tab === "entry" ? "entry" : "exit"}-pages?limit=10&${qs}`)
      .then((r) => alive && setRows(r || []))
      .catch(() => alive && setRows([]));
    return () => {
      alive = false;
    };
  }, [basePath, qs, tab]);

  const total = rows.reduce((a, r) => a + r.value, 0);
  return (
    <Card title={tab === "entry" ? "Entry pages" : "Exit pages"} icon={<IconGlobe className="h-4 w-4 text-indigo-500" />}>
      <div className="mb-3 flex rounded-lg border border-edge bg-raised/40 p-1 text-xs">
        {(["entry", "exit"] as const).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`rounded-md px-3 py-1 transition-colors ${
              tab === t ? "bg-raised font-medium text-ink" : "text-faint hover:text-ink"
            }`}
          >
            {t === "entry" ? "Landing" : "Leaving"}
          </button>
        ))}
      </div>
      {rows.length === 0 ? (
        <p className="text-xs text-faint">No data in this period.</p>
      ) : (
        <div className="space-y-2">
          {rows.map((r) => (
            <button
              key={r.key}
              onClick={() => setFilter("page", r.key)}
              className="group block w-full text-left"
              title={`Filter by ${r.key}`}
            >
              <div className="flex items-baseline justify-between gap-3">
                <span className="min-w-0 flex-1 truncate text-sm text-soft group-hover:text-ink">{r.key || "/"}</span>
                <span className="text-xs text-faint">{total > 0 ? ((r.value / total) * 100).toFixed(0) : 0}%</span>
                <span className="w-10 text-right text-sm font-medium text-ink">{r.value}</span>
              </div>
              <div className="mt-1 h-1 w-full overflow-hidden rounded-full bg-raised">
                <div
                  className="h-full rounded-full bg-indigo-500"
                  style={{ width: `${total > 0 ? (r.value / total) * 100 : 0}%` }}
                />
              </div>
            </button>
          ))}
        </div>
      )}
    </Card>
  );
}