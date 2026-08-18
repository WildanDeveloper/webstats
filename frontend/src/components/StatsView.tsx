"use client";

import { useRouter, usePathname } from "next/navigation";
import Link from "next/link";
import type { Overview, TimePoint, Row, EventRow, Site } from "@/lib/types";
import StatCards from "@/components/charts/StatCards";
import VisitorsChart from "@/components/charts/VisitorsChart";
import TopList from "@/components/charts/TopList";
import DonutChart from "@/components/charts/DonutChart";
import { IconArrowLeft, IconGlobe, IconPulse } from "@/components/icons";

const PERIODS = [
  { key: "24h", label: "24 jam" },
  { key: "7d", label: "7 hari" },
  { key: "30d", label: "30 hari" },
  { key: "all", label: "Semua" },
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
    <section className="rounded-xl border border-zinc-800 bg-zinc-900/60 p-5">
      <h2 className="mb-4 flex items-center gap-2 text-sm font-semibold text-zinc-300">
        {icon}
        {title}
      </h2>
      {children}
    </section>
  );
}

export default function StatsView(props: {
  site: Site | null;
  period: string;
  overview: Overview | null;
  timeseries: TimePoint[];
  pages: Row[];
  referrers: Row[];
  devices: Row[];
  browsers: Row[];
  os: Row[];
  countries: Row[];
  events: EventRow[];
  error: string;
}) {
  const router = useRouter();
  const pathname = usePathname();

  return (
    <div className="space-y-6">
      <header>
        <Link
          href="/"
          className="inline-flex items-center gap-1.5 text-sm text-zinc-500 transition-colors hover:text-zinc-300"
        >
          <IconArrowLeft className="h-3.5 w-3.5" />
          Situs
        </Link>
        <div className="mt-1 flex flex-wrap items-center justify-between gap-4">
          <div>
            <h1 className="text-2xl font-semibold tracking-tight text-zinc-100">
              {props.site?.name || "Situs"}
            </h1>
            <p className="mt-0.5 flex items-center gap-1.5 text-sm text-zinc-500">
              <IconGlobe className="h-3.5 w-3.5" />
              {props.site?.domain || "tanpa domain"}
            </p>
          </div>
          <div className="flex rounded-lg border border-zinc-800 bg-zinc-900/60 p-1">
            {PERIODS.map((p) => (
              <button
                key={p.key}
                onClick={() => router.push(`${pathname}?period=${p.key}`)}
                className={`rounded-md px-3.5 py-1.5 text-sm transition-colors ${
                  props.period === p.key
                    ? "bg-zinc-700/80 font-medium text-zinc-100"
                    : "text-zinc-500 hover:text-zinc-200"
                }`}
              >
                {p.label}
              </button>
            ))}
          </div>
        </div>
      </header>

      {props.error ? (
        <div className="rounded-xl border border-red-900/60 bg-red-950/40 p-6 text-sm text-red-300">
          Gagal memuat data: {props.error}
        </div>
      ) : (
        <>
          <StatCards
            pageviews={props.overview?.pageviews ?? 0}
            visitors={props.overview?.visitors ?? 0}
            bounceRate={props.overview?.bounce_rate ?? 0}
            avgPerDay={props.overview?.avg_per_day ?? 0}
          />

          <Card title="Tren pengunjung" icon={<IconPulse className="h-4 w-4 text-indigo-400" />}>
            <VisitorsChart data={props.timeseries} />
          </Card>

          <div className="grid gap-6 lg:grid-cols-2">
            <TopList title="Halaman teratas" rows={props.pages} />
            <TopList title="Referrer" rows={props.referrers} />
            <DonutChart title="Perangkat" rows={props.devices} />
            <DonutChart title="Browser" rows={props.browsers} />
            <DonutChart title="Sistem operasi" rows={props.os} />
            <DonutChart title="Negara" rows={props.countries} />
          </div>

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