"use client";

import { PieChart, Pie, Cell, Tooltip, ResponsiveContainer } from "recharts";
import type { Row } from "@/lib/types";

const nf = new Intl.NumberFormat("id-ID");

const COLORS = [
  "#6366f1",
  "#34d399",
  "#fbbf24",
  "#22d3ee",
  "#f472b6",
  "#a78bfa",
  "#fb7185",
  "#4ade80",
  "#facc15",
  "#818cf8",
];

function Tip({ active, payload }: any) {
  if (!active || !payload?.length) return null;
  const d = payload[0];
  return (
    <div className="rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-2 text-xs shadow-xl">
      <span className="font-medium text-zinc-200">{d.name}</span>
      <span className="ml-2 text-zinc-400">
        {nf.format(d.value)} ({d.payload.pct}%)
      </span>
    </div>
  );
}

export default function DonutChart({ title, rows }: { title: string; rows: Row[] }) {
  if (!rows.length) return null;
  const total = rows.reduce((s, r) => s + r.value, 0);
  const data = rows.slice(0, 8).map((r, i) => ({
    name: r.key,
    value: r.value,
    pct: ((r.value / total) * 100).toFixed(1),
    color: COLORS[i % COLORS.length],
  }));

  return (
    <section className="rounded-xl border border-zinc-800 bg-zinc-900/60 p-5">
      <h2 className="mb-2 text-sm font-semibold text-zinc-300">{title}</h2>
      <div className="flex items-center gap-2">
        <ResponsiveContainer width="55%" height={210}>
          <PieChart>
            <Pie
              data={data}
              dataKey="value"
              nameKey="name"
              innerRadius={58}
              outerRadius={82}
              paddingAngle={2}
              strokeWidth={0}
            >
              {data.map((d) => (
                <Cell key={d.name} fill={d.color} />
              ))}
            </Pie>
            <Tooltip content={<Tip />} />
          </PieChart>
        </ResponsiveContainer>
        <ul className="min-w-0 flex-1 space-y-2">
          {data.map((d) => (
            <li key={d.name} className="flex items-center gap-2 text-xs">
              <span
                className="h-2 w-2 shrink-0 rounded-full"
                style={{ background: d.color }}
              />
              <span className="min-w-0 flex-1 truncate text-zinc-300">{d.name}</span>
              <span className="shrink-0 tabular-nums text-zinc-500">{d.pct}%</span>
            </li>
          ))}
        </ul>
      </div>
    </section>
  );
}