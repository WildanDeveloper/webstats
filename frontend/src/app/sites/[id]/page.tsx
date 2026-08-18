import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import { authOptions, apiFetch } from "@/lib/auth";
import type { Overview, TimePoint, Row, EventRow, Site } from "@/lib/types";
import AppShell from "@/components/AppShell";
import StatsView from "@/components/StatsView";

export const dynamic = "force-dynamic";

export default async function SitePage({
  params,
  searchParams,
}: {
  params: { id: string };
  searchParams: { period?: string };
}) {
  const session = await getServerSession(authOptions);
  if (!session?.token) redirect("/login");

  const period = searchParams.period || "7d";
  const q = `period=${period}`;

  let site: Site | null = null;
  let overview: Overview | null = null;
  let timeseries: TimePoint[] = [];
  let pages: Row[] = [];
  let referrers: Row[] = [];
  let devices: Row[] = [];
  let browsers: Row[] = [];
  let os: Row[] = [];
  let countries: Row[] = [];
  let events: EventRow[] = [];
  let error = "";

  try {
    [site, overview, timeseries, pages, referrers, devices, browsers, os, countries, events] =
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
        apiFetch<EventRow[]>(`/api/sites/${params.id}/events?${q}`, session.token),
      ]);
  } catch (e: any) {
    error = e.message || "Gagal mengambil data";
  }

  return (
    <AppShell
      name={session.user?.name || ""}
      email={session.user?.email || ""}
      token={session.token}
    >
      <div className="mx-auto max-w-6xl px-6 py-8 lg:px-10">
        <StatsView
          site={site}
          period={period}
          overview={overview}
          timeseries={timeseries}
          pages={pages}
          referrers={referrers}
          devices={devices}
          browsers={browsers}
          os={os}
          countries={countries}
          events={events}
          error={error}
        />
      </div>
    </AppShell>
  );
}