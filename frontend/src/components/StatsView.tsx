"use client";

import { useRouter, usePathname } from "next/navigation";
import Link from "next/link";
import type { Overview, TimePoint, Row, EventRow, Site } from "@/lib/types";
import StatCards from "@/components/charts/StatCards";
import VisitorsChart from "@/components/charts/VisitorsChart";
import TopList from "@/components/charts/TopList";
import DonutChart from "@/components/charts/DonutChart";
import {
  IconArrowLeft,
  IconGlobe,
  IconPulse,
  IconSettings,
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
            </h1>
            <p className="mt-0.5 flex items-center gap-1.5 text-sm text-faint">
              <IconGlobe className="h-3.5 w-3.5" />
              {props.site?.domain || "no domain"}
            </p>
          </div>
          <div className="flex rounded-lg border border-edge bg-card p-1">
            {PERIODS.map((p) => (
              <button
                key={p.key}
                onClick={() => router.push(`${pathname}?period=${p.key}`)}
                className={`rounded-md px-3.5 py-1.5 text-sm transition-colors ${
                  props.period === p.key
                    ? "bg-raised font-medium text-ink"
                    : "text-faint hover:text-ink"
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
          Failed to load data: {props.error}
        </div>
      ) : (
        <>
          <StatCards
            pageviews={props.overview?.pageviews ?? 0}
            visitors={props.overview?.visitors ?? 0}
            bounceRate={props.overview?.bounce_rate ?? 0}
            avgPerDay={props.overview?.avg_per_day ?? 0}
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