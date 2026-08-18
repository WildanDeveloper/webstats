import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import { authOptions, apiFetch } from "@/lib/auth";
import type {
  Campaign,
  EventDetail,
  GoalSummary,
  Overview,
  Row,
  Site,
  TimePoint,
  WorldPoint,
} from "@/lib/types";
import AppShell from "@/components/AppShell";
import StatsView from "@/components/StatsView";

export const dynamic = "force-dynamic";

export default async function SitePage({
  params,
  searchParams,
}: {
  params: { id: string };
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
  const session = await getServerSession(authOptions);
  if (!session?.token) redirect("/login");

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

  let site: Site | null = null;
  let overview: Overview | null = null;
  let timeseries: TimePoint[] = [];
  let pages: Row[] = [];
  let referrers: Row[] = [];
  let devices: Row[] = [];
  let browsers: Row[] = [];
  let os: Row[] = [];
  let countries: Row[] = [];
  let events: EventDetail[] = [];
  let world: WorldPoint[] = [];
  let campaigns: Campaign[] = [];
  let goals: GoalSummary[] = [];
  let error = "";

  try {
    [site, overview, timeseries, pages, referrers, devices, browsers, os, countries, events, world, campaigns, goals] =
      await Promise.all([
        apiFetch<Site>(`/api/sites/${params.id}`, session.token),
        apiFetch<Overview>(`/api/sites/${params.id}/overview?${q}`, session.token),
        apiFetch<TimePoint[]>(`/api/sites/${params.id}/timeseries?${q}`, session.token),
        apiFetch<Row[]>(`/api/sites/${params.id}/pages?${q}`, session.token),
        apiFetch<Row[]>(`/api/sites/${params.id}/referrers?${q}`, session.token),
        apiFetch<Row[]>(`/api/sites/${params.id}/devices?${q}`, session.token),
        apiFetch<Row[]>(`/api/sites/${params.id}/browsers?${q}`, session.token),
        apiFetch<Row[]>(`/api/sites/${params.id}/os?${q}`, session.token),
        apiFetch<Row[]>(`/api/sites/${params.id}/countries?${q}`, session.token),
        apiFetch<EventDetail[]>(`/api/sites/${params.id}/events/detail?${q}`, session.token),
        apiFetch<WorldPoint[]>(`/api/sites/${params.id}/world?${q}`, session.token),
        apiFetch<Campaign[]>(`/api/sites/${params.id}/campaigns?${q}`, session.token),
        apiFetch<GoalSummary[]>(`/api/sites/${params.id}/goals/summary?${q}`, session.token),
      ]);
  } catch (e: any) {
    error = e.message || "Failed to load data";
  }

  return (
    <AppShell
      name={session.user?.name || ""}
      email={session.user?.email || ""}
      role={session.user?.role || "user"}
    >
      <div className="mx-auto max-w-6xl px-6 py-8 lg:px-10">
        <StatsView
          site={site}
          siteId={params.id}
          token={session.token}
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
          error={error}
        />
      </div>
    </AppShell>
  );
}