"use client";

import type { Row } from "@/lib/types";
import { IconChart, IconLink, IconMonitor, IconBrowser, IconCpu, IconFlag } from "@/components/icons";

const nf = new Intl.NumberFormat("en-US");

const ICONS: Record<string, React.ComponentType<{ className?: string }>> = {
  "Top pages": IconChart,
  Referrers: IconLink,
  Devices: IconMonitor,
  Browsers: IconBrowser,
  "Operating systems": IconCpu,
  Countries: IconFlag,
};

export default function TopList({ title, rows }: { title: string; rows: Row[] }) {
  if (!rows.length) return null;
  const max = Math.max(...rows.map((r) => r.value), 1);
  const Icon = ICONS[title];

  return (
    <section className="rounded-xl border border-edge bg-card p-5">
      <h2 className="mb-4 flex items-center gap-2 text-sm font-semibold text-ink">
        {Icon && <Icon className="h-4 w-4 text-indigo-500" />}
        {title}
      </h2>
      {title === "Top pages" || title === "Referrers" ? (
        <table className="w-full text-sm">
          <tbody>
            {rows.map((r, i) => (
              <tr key={r.key} className="border-b border-edge last:border-0">
                <td className="w-8 py-2.5 pr-2 text-xs tabular-nums text-faint">
                  {String(i + 1).padStart(2, "0")}
                </td>
                <td className="w-full py-2.5 pr-4">
                  <div className="flex items-center gap-3">
                    <span className="max-w-[240px] truncate text-soft">
                      {r.key}
                    </span>
                    <span className="h-1 min-w-6 max-w-36 flex-1 overflow-hidden rounded-full bg-raised">
                      <span
                        className="block h-full rounded-full bg-indigo-500/80"
                        style={{ width: `${(r.value / max) * 100}%` }}
                      />
                    </span>
                  </div>
                </td>
                <td className="py-2.5 text-right font-medium tabular-nums text-ink">
                  {nf.format(r.value)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      ) : (
        <ul className="space-y-2.5">
          {rows.map((r) => (
            <li key={r.key} className="flex items-center gap-3 text-sm">
              <span className="max-w-[180px] truncate text-soft">{r.key}</span>
              <span className="h-1 flex-1 overflow-hidden rounded-full bg-raised">
                <span
                  className="block h-full rounded-full bg-indigo-500/80"
                  style={{ width: `${(r.value / max) * 100}%` }}
                />
              </span>
              <span className="w-14 text-right font-medium tabular-nums text-ink">
                {nf.format(r.value)}
              </span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}