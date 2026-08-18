"use client";

import {
  ResponsiveContainer,
  AreaChart,
  Area,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
} from "recharts";
import type { SiteSeries } from "@/lib/types";

const nf = new Intl.NumberFormat("en-US", { notation: "compact" });

function Tip({ active, payload, label }: any) {
  if (!active || !payload?.length) return null;
  return (
    <div
      className="rounded-lg border px-3.5 py-2.5 text-xs shadow-xl"
      style={{
        background: "var(--tip-bg)",
        borderColor: "var(--tip-border)",
      }}
    >
      <p className="mb-1.5 font-medium" style={{ color: "var(--soft)" }}>
        {label}
      </p>
      {payload.map((p: any) => (
        <div key={p.dataKey} className="flex items-center gap-2 py-0.5">
          <span
            className="h-2 w-2 rounded-full"
            style={{ background: p.color || p.stroke }}
          />
          <span style={{ color: "var(--soft)" }}>{p.name}:</span>
          <span
            className="ml-auto pl-4 font-medium"
            style={{ color: "var(--ink)" }}
          >
            {nf.format(p.value)}
          </span>
        </div>
      ))}
    </div>
  );
}

export default function MultiSiteChart({ series }: { series: SiteSeries[] }) {
  const active = series.filter((s) => s.points.length > 0);

  if (!active.length) {
    return (
      <p className="flex h-64 items-center justify-center text-sm text-faint">
        No traffic in this period yet.
      </p>
    );
  }

  const maps = active.map((s) => {
    const m = new Map<string, number>();
    s.points.forEach((p) => m.set(p.date, p.pageviews));
    return m;
  });
  const data = active[0].points.map((p) => {
    const row: Record<string, any> = { date: p.date };
    active.forEach((s, i) => {
      row[s.site_id] = maps[i].get(p.date) || 0;
    });
    return row;
  });

  return (
    <div>
      <ResponsiveContainer width="100%" height={300}>
        <AreaChart data={data} margin={{ top: 8, right: 8, bottom: 0, left: 0 }}>
          <defs>
            {active.map((s) => (
              <linearGradient
                key={s.site_id}
                id={`grad-${s.site_id}`}
                x1="0"
                y1="0"
                x2="0"
                y2="1"
              >
                <stop offset="0%" stopColor={s.color} stopOpacity={0.22} />
                <stop offset="100%" stopColor={s.color} stopOpacity={0} />
              </linearGradient>
            ))}
          </defs>
          <CartesianGrid strokeDasharray="4 4" vertical={false} />
          <XAxis
            dataKey="date"
            tick={{ fontSize: 11 }}
            tickLine={false}
            axisLine={false}
            dy={6}
          />
          <YAxis
            tick={{ fontSize: 11 }}
            tickLine={false}
            axisLine={false}
            tickFormatter={(v: number) => nf.format(v)}
            width={44}
          />
          <Tooltip content={<Tip />} cursor={{ strokeDasharray: "4 4" }} />
          {active.map((s) => (
            <Area
              key={s.site_id}
              type="monotone"
              dataKey={s.site_id}
              name={s.name}
              stroke={s.color || "#6366f1"}
              strokeWidth={2}
              fill={`url(#grad-${s.site_id})`}
              dot={false}
              activeDot={{ r: 3.5, strokeWidth: 0 }}
            />
          ))}
        </AreaChart>
      </ResponsiveContainer>
      <div className="mt-3 flex flex-wrap items-center gap-x-5 gap-y-2 text-xs text-soft">
        {active.map((s) => (
          <span key={s.site_id} className="flex items-center gap-1.5">
            <span
              className="h-2 w-2 rounded-full"
              style={{ background: s.color || "#6366f1" }}
            />
            {s.name}
          </span>
        ))}
      </div>
    </div>
  );
}