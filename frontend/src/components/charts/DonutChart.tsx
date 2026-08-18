"use client";

import { PieChart, Pie, Cell, Tooltip, ResponsiveContainer } from "recharts";
import type { Row } from "@/lib/types";

const nf = new Intl.NumberFormat("en-US");

const COLORS = [
  "#6366f1",
  "#10b981",
  "#f59e0b",
  "#06b6d4",
  "#ec4899",
  "#8b5cf6",
  "#ef4444",
  "#84cc16",
  "#facc15",
  "#3b82f6",
];

function Tip({ active, payload }: any) {
  if (!active || !payload?.length) return null;
  const d = payload[0];
  return (
    <div
      className="rounded-lg border px-3 py-2 text-xs shadow-xl"
      style={{
        background: "var(--tip-bg)",
        borderColor: "var(--tip-border)",
      }}
    >
      <span style={{ color: "var(--ink)", fontWeight: 500 }}>{d.name}</span>
      <span className="ml-2" style={{ color: "var(--soft)" }}>
        {nf.format(d.value)} ({d.payload.pct}%)
      </span>
    </div>
  );
}

export default function DonutChart({
  title,
  rows,
  onSelect,
}: {
  title: string;
  rows: Row[];
  onSelect?: (key: string) => void;
}) {
  if (!rows.length) return null;
  const total = rows.reduce((s, r) => s + r.value, 0);
  const data = rows.slice(0, 8).map((r, i) => ({
    name: r.key,
    value: r.value,
    pct: ((r.value / total) * 100).toFixed(1),
    color: COLORS[i % COLORS.length],
  }));

  return (
    <section className="rounded-xl border border-edge bg-card p-5">
      <h2 className="mb-2 text-sm font-semibold text-ink">{title}</h2>
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
            <li
              key={d.name}
              onClick={() => onSelect?.(d.name)}
              className={`flex items-center gap-2 text-xs ${onSelect ? "cursor-pointer" : ""}`}
            >
              <span
                className="h-2 w-2 shrink-0 rounded-full"
                style={{ background: d.color }}
              />
              <span className={`min-w-0 flex-1 truncate ${onSelect ? "text-soft hover:text-indigo-400" : "text-soft"}`}>{d.name}</span>
              <span className="shrink-0 tabular-nums text-faint">{d.pct}%</span>
            </li>
          ))}
        </ul>
      </div>
    </section>
  );
}