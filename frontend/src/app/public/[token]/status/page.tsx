import Link from "next/link";
import type { PublicStatus } from "@/lib/types";
import { IconArrowLeft, IconPulse, IconShieldCheck, IconShieldX } from "@/components/icons";

export const dynamic = "force-dynamic";

export default async function PublicStatusPage({
  params,
}: {
  params: { token: string };
}) {
  const base = process.env.NEXT_PUBLIC_API_URL || "http://127.0.0.1:8086";
  let status: PublicStatus | null = null;
  try {
    const res = await fetch(`${base}/api/public/${params.token}/status`, { cache: "no-store" });
    if (res.ok) status = (await res.json()) as PublicStatus;
  } catch {
    status = null;
  }

  if (!status) {
    return (
      <div className="min-h-screen bg-bg">
        <div className="mx-auto mt-16 w-full max-w-md px-6">
          <div className="rounded-xl border border-red-900/60 bg-red-950/40 p-8 text-center">
            <h1 className="text-lg font-semibold text-red-300">Status page not found</h1>
            <p className="mt-2 text-sm text-red-300/80">
              This status page does not exist or is not public.
            </p>
          </div>
        </div>
      </div>
    );
  }

  const okCount = status.monitors.filter((m) => m.last_ok).length;
  const overall = status.monitors.length > 0 ? Math.round((okCount / status.monitors.length) * 100) : 100;

  return (
    <div className="min-h-screen bg-bg">
      <div className="mx-auto max-w-2xl px-6 py-10 lg:px-10">
        <header>
          <Link
            href={`/public/${params.token}`}
            className="inline-flex items-center gap-1.5 text-sm text-faint transition-colors hover:text-ink"
          >
            <IconArrowLeft className="h-3.5 w-3.5" />
            Back to dashboard
          </Link>
          <div className="mt-2 flex items-center gap-3">
            <span
              className="h-3 w-3 rounded-full"
              style={{ background: status.site.color || "#6366f1" }}
            />
            <h1 className="text-2xl font-semibold tracking-tight text-ink">
              {status.site.name} status
            </h1>
          </div>
          <p className="mt-1 text-sm text-faint">{status.site.domain}</p>
        </header>

        <div className="mt-6 rounded-xl border border-edge bg-card p-6">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-medium uppercase tracking-wide text-faint">Uptime</p>
              <p className="mt-1 text-3xl font-semibold text-ink">{overall}%</p>
            </div>
            <span
              className={`flex items-center gap-1.5 rounded-full px-3 py-1.5 text-sm font-medium ${
                overall === 100 ? "bg-emerald-500/10 text-emerald-500" : "bg-amber-500/10 text-amber-500"
              }`}
            >
              <IconPulse className="h-4 w-4" />
              {overall === 100 ? "All systems operational" : "Degraded"}
            </span>
          </div>
        </div>

        <div className="mt-4 space-y-3">
          {status.monitors.length === 0 && (
            <p className="rounded-xl border border-edge bg-card p-6 text-sm text-faint">
              No monitors configured for this site.
            </p>
          )}
          {status.monitors.map((m) => (
            <div key={m.id} className="rounded-xl border border-edge bg-card p-4">
              <div className="flex items-center gap-4">
                <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-raised">
                  {m.last_ok ? (
                    <IconShieldCheck className="h-5 w-5 text-emerald-500" />
                  ) : (
                    <IconShieldX className="h-5 w-5 text-red-500" />
                  )}
                </span>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium text-ink">{m.url}</p>
                  <p className="text-xs text-faint">
                    {m.last_ok
                      ? m.last_status
                        ? `HTTP ${m.last_status}`
                        : "Up"
                      : m.last_status
                        ? `HTTP ${m.last_status}`
                        : "Down"}
                    {m.last_check_at
                      ? ` · checked ${new Date(m.last_check_at).toLocaleString("en-US", {
                          month: "short",
                          day: "numeric",
                          hour: "2-digit",
                          minute: "2-digit",
                        })}`
                      : " · waiting for first check"}
                  </p>
                </div>
                <span
                  className={`text-sm font-semibold ${
                    m.uptime_pct >= 99 ? "text-emerald-500" : m.uptime_pct >= 90 ? "text-amber-500" : "text-red-500"
                  }`}
                >
                  {m.uptime_pct.toFixed(2)}%
                </span>
              </div>
              {m.days && m.days.length > 0 && <UptimeBars days={m.days} />}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

const DAY = 24 * 60 * 60 * 1000;

function UptimeBars({ days }: { days: NonNullable<PublicStatus["monitors"][number]["days"]> }) {
  const byDate = new Map(days.map((d) => [d.date, d]));
  const today = new Date();
  const bars: React.ReactNode[] = [];
  for (let i = 89; i >= 0; i--) {
    const date = new Date(Date.UTC(today.getUTCFullYear(), today.getUTCMonth(), today.getUTCDate() - i));
    const key = date.toISOString().slice(0, 10);
    const d = byDate.get(key);
    const total = d?.total ?? 0;
    const up = d?.up ?? 0;
    const cls =
      total === 0
        ? "bg-raised"
        : up / total >= 1
          ? "bg-emerald-500"
          : up / total >= 0.9
            ? "bg-amber-500"
            : "bg-red-500";
    bars.push(
      <span
        key={key}
        title={
          total > 0
            ? `${key}: ${up}/${total} checks up`
            : `${key}: no checks`
        }
        className={`h-6 flex-1 rounded-[2px] ${cls}`}
        style={{ minWidth: 3 }}
      />,
    );
  }
  return (
    <div className="mt-3">
      <div className="flex gap-[2px]">{bars}</div>
      <div className="mt-1.5 flex justify-between text-[10px] text-faint">
        <span>90 days ago</span>
        <span>today</span>
      </div>
    </div>
  );
}