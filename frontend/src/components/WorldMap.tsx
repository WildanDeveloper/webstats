"use client";

import type { WorldPoint } from "@/lib/types";
import { COUNTRY_PATHS } from "./countryPaths";

export default function WorldMap({ points }: { points: WorldPoint[] }) {
  if (points.length === 0) return null;

  const max = Math.max(...points.map((p) => p.count));
  const byCode = new Map(points.map((p) => [p.country, p.count]));

  return (
    <section className="rounded-xl border border-edge bg-card p-5">
      <h2 className="mb-4 flex items-center gap-2 text-sm font-semibold text-ink">
        World map
      </h2>
      <svg viewBox="0 0 720 360" className="w-full">
        {Object.entries(COUNTRY_PATHS).map(([cc, d]) => {
          const n = byCode.get(cc);
          return (
            <path
              key={cc}
              d={d}
              fill={n ? "#6366f1" : "currentColor"}
              fillOpacity={n ? 0.18 + 0.82 * (n / max) : 0.45}
              className={n ? undefined : "text-edge"}
              style={{ transition: "fill-opacity 150ms" }}
            >
              <title>
                {cc}
                {n ? ` — ${n} pageview${n > 1 ? "s" : ""}` : ""}
              </title>
            </path>
          );
        })}
      </svg>
    </section>
  );
}