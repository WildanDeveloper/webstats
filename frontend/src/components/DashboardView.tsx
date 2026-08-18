"use client";

import { useRouter, usePathname } from "next/navigation";
import Link from "next/link";
import type { RootOverview, Site } from "@/lib/types";
import MultiSiteChart from "@/components/charts/MultiSiteChart";
import {
  IconMouse,
  IconUsers,
  IconGrid,
  IconPulse,
  IconPlus,
} from "@/components/icons";

const nf = new Intl.NumberFormat("en-US");

const PERIODS = [
  { key: "24h", label: "24h" },
  { key: "7d", label: "7 days" },
  { key: "30d", label: "30 days" },
  { key: "all", label: "All" },
];

export default function DashboardView({
  period,
  overview,
  sites,
  userName,
}: {
  period: string;
  overview: RootOverview | null;
  sites: Site[];
  userName: string;
}) {
  const router = useRouter();
  const pathname = usePathname();

  const totals = { pageviews: 0, visitors: 0, events: 0, sites: 0 };
  if (overview) {
    totals.pageviews = overview.pageviews;
    totals.visitors = overview.visitors;
    totals.events = overview.events;
    totals.sites = overview.sites;
  }

  const perSiteTotal = (id: string) => {
    const s = overview?.series.find((x) => x.site_id === id);
    if (!s) return 0;
    return s.points.reduce((acc, p) => acc + p.pageviews, 0);
  };

  const cards = [
    { label: "Pageviews", value: nf.format(totals.pageviews), icon: IconMouse, tint: "text-indigo-500 bg-indigo-500/10" },
    { label: "Unique visitors", value: nf.format(totals.visitors), icon: IconUsers, tint: "text-emerald-500 bg-emerald-500/10" },
    { label: "Sites", value: nf.format(totals.sites), icon: IconGrid, tint: "text-sky-500 bg-sky-500/10" },
    { label: "Events", value: nf.format(totals.events), icon: IconPulse, tint: "text-amber-500 bg-amber-500/10" },
  ];

  return (
    <div className="space-y-6">
      <header className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-ink">
            Hello, {userName}
          </h1>
          <p className="mt-1 text-sm text-soft">
            Here is how your websites are doing.
          </p>
        </div>
        <div className="flex rounded-lg border border-edge bg-card p-1">
          {PERIODS.map((p) => (
            <button
              key={p.key}
              onClick={() => router.push(`${pathname}?period=${p.key}`)}
              className={`rounded-md px-3.5 py-1.5 text-sm transition-colors ${
                period === p.key
                  ? "bg-raised font-medium text-ink"
                  : "text-faint hover:text-ink"
              }`}
            >
              {p.label}
            </button>
          ))}
        </div>
      </header>

      <div className="grid grid-cols-2 gap-4 xl:grid-cols-4">
        {cards.map((c) => (
          <div
            key={c.label}
            className="rounded-xl border border-edge bg-card p-5"
          >
            <span className={`flex h-9 w-9 items-center justify-center rounded-lg ${c.tint}`}>
              <c.icon className="h-[18px] w-[18px]" />
            </span>
            <p className="mt-3 text-2xl font-semibold tracking-tight text-ink">
              {c.value}
            </p>
            <p className="mt-0.5 text-xs text-faint">{c.label}</p>
          </div>
        ))}
      </div>

      <section className="rounded-xl border border-edge bg-card p-5">
        <h2 className="mb-4 flex items-center gap-2 text-sm font-semibold text-ink">
          <IconPulse className="h-4 w-4 text-indigo-500" />
          Traffic by site
        </h2>
        <MultiSiteChart series={overview?.series || []} />
      </section>

      <section>
        <div className="mb-3 flex items-center justify-between">
          <h2 className="text-sm font-semibold text-ink">Your sites</h2>
          <Link
            href="/sites"
            className="flex items-center gap-1.5 text-sm font-medium text-indigo-500 hover:text-indigo-400"
          >
            <IconPlus className="h-4 w-4" />
            Manage sites
          </Link>
        </div>
        {sites.length === 0 ? (
          <div className="rounded-xl border border-dashed border-edge py-14 text-center">
            <p className="text-sm font-medium text-ink">No sites yet</p>
            <p className="mt-1 text-sm text-faint">
              Add your first website to start collecting data.
            </p>
            <Link
              href="/sites"
              className="mt-4 inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-500"
            >
              <IconPlus className="h-4 w-4" />
              Add a site
            </Link>
          </div>
        ) : (
          <div className="overflow-hidden rounded-xl border border-edge bg-card">
            <table className="w-full text-sm">
              <tbody>
                {sites.map((s) => (
                  <tr key={s.id} className="border-b border-edge last:border-0">
                    <td className="w-14 py-3.5 pl-5">
                      <span
                        className="block h-3 w-3 rounded-full"
                        style={{ background: s.color || "#6366f1" }}
                      />
                    </td>
                    <td className="py-3.5">
                      <div className="flex items-center gap-2.5">
                        <span className="font-medium text-ink">{s.name}</span>
                        <span className="rounded-full border border-edge px-2 py-0.5 text-[11px] text-faint">
                          {s.domain || "no domain"}
                        </span>
                      </div>
                    </td>
                    <td className="py-3.5 text-right tabular-nums text-soft">
                      {nf.format(perSiteTotal(s.id))} pageviews
                    </td>
                    <td className="w-24 py-3.5 pr-5 text-right">
                      <Link
                        href={`/sites/${s.id}`}
                        className="text-indigo-500 hover:text-indigo-400"
                      >
                        View stats
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}