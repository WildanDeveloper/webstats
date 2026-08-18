"use client";

import type { WorldPoint } from "@/lib/types";

export default function WorldMap({ points }: { points: WorldPoint[] }) {
  if (points.length === 0) return null;

  const W = 720;
  const H = 360;
  const max = Math.max(...points.map((p) => p.count));
  const x = (lng: number) => ((lng + 180) / 360) * W;
  const y = (lat: number) => ((90 - lat) / 180) * H;

  return (
    <section className="rounded-xl border border-edge bg-card p-5">
      <h2 className="mb-4 flex items-center gap-2 text-sm font-semibold text-ink">
        World map
      </h2>
      <svg viewBox={`0 0 ${W} ${H}`} className="w-full">
        {points.map((p) => {
          const r = 3 + (p.count / max) * 9;
          return (
            <circle
              key={p.country}
              cx={x(p.lng)}
              cy={y(p.lat)}
              r={r}
              fill="#6366f1"
              opacity={0.75}
              className="cursor-pointer transition-opacity hover:opacity-100"
            >
              <title>{`${p.country} — ${p.count} pageview${p.count > 1 ? "s" : ""}`}</title>
            </circle>
          );
        })}
        {points.slice(0, 8).map((p) => (
          <text
            key={"t" + p.country}
            x={Math.min(Math.max(x(p.lng) + 8, 4), W - 60)}
            y={Math.max(y(p.lat) - 4, 10)}
            className="fill-faint"
            fontSize="9"
          >
            {`${p.country} ${p.count}`}
          </text>
        ))}
      </svg>
    </section>
  );
}