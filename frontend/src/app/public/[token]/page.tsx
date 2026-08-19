import type {
  Campaign,
  EventDetail,
  FunnelStep,
  GoalSummary,
  Insights,
  Overview,
  PublicStatus,
  Row,
  Site,
  TimePoint,
  WorldPoint,
} from "@/lib/types";
import StatsView from "@/components/StatsView";

export const dynamic = "force-dynamic";

export default async function PublicDashboardPage({
  params,
  searchParams,
}: {
  params: { token: string };
  searchParams: {
    period?: string;
    from?: string;
    to?: string;
    page?: string;
    source?: string;
    country?: string;
    device?: string;
    browser?: string;
    os?: string;
  };
}) {
  const base = process.env.NEXT_PUBLIC_API_URL || "http://127.0.0.1:8086";
  const token = params.token;
  const period = searchParams.period || "7d";
  const from = searchParams.from || "";
  const to = searchParams.to || "";
  const filters = {
    page: searchParams.page || "",
    source: searchParams.source || "",
    country: searchParams.country || "",
    device: searchParams.device || "",
    browser: searchParams.browser || "",
    os: searchParams.os || "",
  };
  const parts: string[] = [];
  if (from && to) {
    parts.push(`from=${from}`, `to=${to}`);
  } else {
    parts.push(`period=${period}`);
  }
  for (const [k, v] of Object.entries(filters)) {
    if (v) parts.push(`${k}=${encodeURIComponent(v)}`);
  }
  const q = parts.join("&");
  const pub = (path: string) => `${base}/api/public/${token}${path}`;

  async function get<T>(path: string, fallback: T): Promise<T> {
    try {
      const res = await fetch(pub(path), { cache: "no-store" });
      if (!res.ok) return fallback;
      return (await res.json()) as T;
    } catch {
      return fallback;
    }
  }

  const [status, overview, timeseries, pages, referrers, devices, browsers, os, countries, events, world, campaigns, goals, insights, funnel] =
    await Promise.all([
      get<PublicStatus | null>(`/status`, null),
      get<Overview | null>(`/overview?${q}`, null),
      get<TimePoint[]>(`/timeseries?${q}`, []),
      get<Row[]>(`/pages?${q}`, []),
      get<Row[]>(`/referrers?${q}`, []),
      get<Row[]>(`/devices?${q}`, []),
      get<Row[]>(`/browsers?${q}`, []),
      get<Row[]>(`/os?${q}`, []),
      get<Row[]>(`/countries?${q}`, []),
      get<EventDetail[]>(`/events/detail?${q}`, []),
      get<WorldPoint[]>(`/world?${q}`, []),
      get<Campaign[]>(`/campaigns?${q}`, []),
      get<GoalSummary[]>(`/goals?${q}`, []),
      get<Insights | null>(`/insights?${q}`, null),
      get<{ report: { steps: FunnelStep[] } }>(`/funnel?${q}`, { report: { steps: [] } }),
    ]);

  const site: Site | null = status
    ? {
        id: "",
        user_id: "",
        name: status.site.name,
        domain: status.site.domain,
        site_key: "",
        color: status.site.color,
        created_at: "",
      }
    : null;

  return (
    <div className="min-h-screen bg-bg">
      <div className="mx-auto max-w-6xl px-6 py-8 lg:px-10">
        <StatsView
          site={site}
          siteId={token}
          token=""
          publicToken={token}
          period={period}
          from={from}
          to={to}
          filters={filters}
          overview={overview}
          timeseries={timeseries}
          pages={pages}
          referrers={referrers}
          devices={devices}
          browsers={browsers}
          os={os}
          countries={countries}
          events={events}
          world={world}
          campaigns={campaigns}
          goals={goals}
          insights={insights}
          funnelReport={funnel?.report?.steps || []}
          error={overview === null ? "dashboard not found" : ""}
        />
      </div>
    </div>
  );
}