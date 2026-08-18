"use client";

import { IconMouse, IconUsers, IconPercent, IconCalendar } from "@/components/icons";

const fmt = new Intl.NumberFormat("en-US");

export default function StatCards({
  pageviews,
  visitors,
  bounceRate,
  avgPerDay,
}: {
  pageviews: number;
  visitors: number;
  bounceRate: number;
  avgPerDay: number;
}) {
  const cards = [
    {
      label: "Pageviews",
      value: fmt.format(pageviews),
      icon: IconMouse,
      tint: "text-indigo-500 bg-indigo-500/10",
    },
    {
      label: "Unique visitors",
      value: fmt.format(visitors),
      icon: IconUsers,
      tint: "text-emerald-500 bg-emerald-500/10",
    },
    {
      label: "Bounce rate",
      value: bounceRate.toFixed(1) + "%",
      icon: IconPercent,
      tint: "text-amber-500 bg-amber-500/10",
    },
    {
      label: "Average per day",
      value: avgPerDay.toFixed(1),
      icon: IconCalendar,
      tint: "text-sky-500 bg-sky-500/10",
    },
  ];

  return (
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
  );
}