"use client";

import {
  IconMouse,
  IconUsers,
  IconPercent,
  IconCalendar,
  IconArrowUp,
  IconArrowDown,
} from "@/components/icons";

const fmt = new Intl.NumberFormat("en-US");

function delta(current: number, previous: number) {
  if (previous <= 0) return null;
  return ((current - previous) / previous) * 100;
}

function Delta({ d, invert }: { d: number | null; invert?: boolean }) {
  if (d === null) return null;
  const up = d >= 0;
  const good = invert ? !up : up;
  const color = good ? "text-emerald-500" : "text-red-400";
  return (
    <span className={`inline-flex items-center gap-0.5 text-[11px] font-medium ${color}`}>
      {up ? <IconArrowUp className="h-3 w-3" /> : <IconArrowDown className="h-3 w-3" />}
      {Math.abs(d).toFixed(1)}%
    </span>
  );
}

export default function StatCards({
  pageviews,
  visitors,
  bounceRate,
  prevPageviews,
  prevVisitors,
  prevBounceRate,
  prevSessions,
  sessions,
}: {
  pageviews: number;
  visitors: number;
  bounceRate: number;
  prevPageviews: number;
  prevVisitors: number;
  prevBounceRate?: number;
  prevSessions?: number;
  sessions?: number;
}) {
  const cards = [
    {
      label: "Pageviews",
      value: fmt.format(pageviews),
      icon: IconMouse,
      tint: "text-indigo-500 bg-indigo-500/10",
      delta: delta(pageviews, prevPageviews),
    },
    {
      label: "Unique visitors",
      value: fmt.format(visitors),
      icon: IconUsers,
      tint: "text-emerald-500 bg-emerald-500/10",
      delta: delta(visitors, prevVisitors),
    },
    {
      label: "Bounce rate",
      value: bounceRate.toFixed(1) + "%",
      icon: IconPercent,
      tint: "text-amber-500 bg-amber-500/10",
      delta:
        prevBounceRate !== undefined && prevBounceRate > 0
          ? ((bounceRate - prevBounceRate) / prevBounceRate) * 100
          : null,
      invert: true,
    },
    {
      label: "Sessions",
      value: fmt.format(sessions ?? 0),
      icon: IconCalendar,
      tint: "text-sky-500 bg-sky-500/10",
      delta: delta(sessions ?? 0, prevSessions ?? 0),
    },
  ];

  return (
    <div className="grid grid-cols-2 gap-4 xl:grid-cols-4">
      {cards.map((c) => (
        <div key={c.label} className="rounded-xl border border-edge bg-card p-5">
          <span className={`flex h-9 w-9 items-center justify-center rounded-lg ${c.tint}`}>
            <c.icon className="h-[18px] w-[18px]" />
          </span>
          <p className="mt-3 text-2xl font-semibold tracking-tight text-ink">
            {c.value}
          </p>
          <p className="mt-0.5 flex items-center gap-1.5 text-xs text-faint">
            {c.delta !== null && <Delta d={c.delta} invert={c.invert} />}
            {c.label}
          </p>
        </div>
      ))}
    </div>
  );
}