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
import type { TimePoint } from "@/lib/types";

const nf = new Intl.NumberFormat("id-ID", { notation: "compact" });

function Tip({ active, payload, label }: any) {
  if (!active || !payload?.length) return null;
  return (
    <div className="rounded-lg border border-zinc-700 bg-zinc-900 px-3.5 py-2.5 text-xs shadow-xl">
      <p className="mb-1.5 font-medium text-zinc-400">{label}</p>
      {payload.map((p: any) => (
        <div key={p.dataKey} className="flex items-center gap-2 py-0.5">
          <span
            className="h-2 w-2 rounded-full"
            style={{ background: p.color || p.stroke }}
          />
          <span className="text-zinc-300">{p.name}:</span>
          <span className="ml-auto pl-4 font-medium text-zinc-100">
            {nf.format(p.value)}
          </span>
        </div>
      ))}
    </div>
  );
}

export default function VisitorsChart({ data }: { data: TimePoint[] }) {
  if (!data.length) {
    return (
      <p className="flex h-64 items-center justify-center text-sm text-zinc-500">
        Belum ada data pada periode ini.
      </p>
    );
  }
  return (
    <div>
      <ResponsiveContainer width="100%" height={300}>
        <AreaChart data={data} margin={{ top: 8, right: 8, bottom: 0, left: 0 }}>
          <defs>
            <linearGradient id="gradPv" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="#6366f1" stopOpacity={0.25} />
              <stop offset="100%" stopColor="#6366f1" stopOpacity={0} />
            </linearGradient>
            <linearGradient id="gradVis" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="#34d399" stopOpacity={0.18} />
              <stop offset="100%" stopColor="#34d399" stopOpacity={0} />
            </linearGradient>
          </defs>
          <CartesianGrid
            stroke="#27272a"
            strokeDasharray="4 4"
            vertical={false}
          />
          <XAxis
            dataKey="date"
            tick={{ fill: "#71717a", fontSize: 11 }}
            tickLine={false}
            axisLine={{ stroke: "#27272a" }}
            dy={6}
          />
          <YAxis
            tick={{ fill: "#71717a", fontSize: 11 }}
            tickLine={false}
            axisLine={false}
            tickFormatter={(v: number) => nf.format(v)}
            width={44}
          />
          <Tooltip content={<Tip />} cursor={{ stroke: "#3f3f46", strokeDasharray: "4 4" }} />
          <Area
            type="monotone"
            dataKey="pageviews"
            name="Pageviews"
            stroke="#818cf8"
            strokeWidth={2}
            fill="url(#gradPv)"
            dot={false}
            activeDot={{ r: 4, strokeWidth: 0 }}
          />
          <Area
            type="monotone"
            dataKey="visitors"
            name="Pengunjung"
            stroke="#34d399"
            strokeWidth={2}
            fill="url(#gradVis)"
            dot={false}
            activeDot={{ r: 4, strokeWidth: 0 }}
          />
        </AreaChart>
      </ResponsiveContainer>
      <div className="mt-3 flex items-center gap-5 text-xs text-zinc-400">
        <span className="flex items-center gap-1.5">
          <span className="h-2 w-2 rounded-full bg-indigo-400" />
          Pageviews
        </span>
        <span className="flex items-center gap-1.5">
          <span className="h-2 w-2 rounded-full bg-emerald-400" />
          Pengunjung unik
        </span>
      </div>
    </div>
  );
}