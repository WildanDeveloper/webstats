"use client";

import { useEffect, useState } from "react";
import { apiFetch, CLIENT_API_URL } from "@/lib/auth";
import type { Realtime } from "@/lib/types";
import { IconBolt, IconMouse, IconUsers } from "@/components/icons";

export default function RealtimePanel({
  token,
  siteId,
}: {
  token: string;
  siteId: string;
}) {
  const [data, setData] = useState<Realtime | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    let es: EventSource | null = null;
    let poll: ReturnType<typeof setInterval> | null = null;

    async function load() {
      try {
        const r = await apiFetch<Realtime>(`/api/sites/${siteId}/realtime`, token);
        if (active) {
          setData(r);
          setError("");
        }
      } catch (e: any) {
        if (active) setError(e.message);
      }
    }

    function startPolling() {
      if (poll || !active) return;
      load();
      poll = setInterval(load, 30000);
    }

    try {
      // Server-Sent Events stream (10s push). Falls back to polling if the
      // browser/proxy can't hold the connection open.
      es = new EventSource(
        `${CLIENT_API_URL}/api/sites/${siteId}/realtime/stream?token=${encodeURIComponent(token)}`,
      );
      es.onmessage = (ev) => {
        if (!active) return;
        try {
          setData(JSON.parse(ev.data));
          setError("");
        } catch {}
      };
      es.onerror = () => {
        es?.close();
        es = null;
        startPolling();
      };
    } catch {
      startPolling();
    }

    return () => {
      active = false;
      es?.close();
      if (poll) clearInterval(poll);
    };
  }, [token, siteId]);

  const active = data?.visitors ?? 0;
  const pvs = data?.pageviews ?? 0;

  return (
    <section className="rounded-xl border border-edge bg-card p-5">
      <h2 className="mb-4 flex items-center gap-2 text-sm font-semibold text-ink">
        <span className="relative flex h-2.5 w-2.5">
          <span
            className={`absolute inline-flex h-full w-full rounded-full opacity-60 ${
              active > 0 ? "animate-ping bg-emerald-400" : "bg-raised"
            }`}
          />
          <span
            className={`relative inline-flex h-2.5 w-2.5 rounded-full ${
              active > 0 ? "bg-emerald-500" : "bg-faint/40"
            }`}
          />
        </span>
        Realtime
      </h2>
      <div className="flex items-center gap-6">
        <div>
          <p className="text-2xl font-semibold tracking-tight text-ink">
            {active}
          </p>
          <p className="mt-0.5 flex items-center gap-1 text-xs text-faint">
            <IconUsers className="h-3 w-3" /> active now
          </p>
        </div>
        <div>
          <p className="text-2xl font-semibold tracking-tight text-ink">{pvs}</p>
          <p className="mt-0.5 flex items-center gap-1 text-xs text-faint">
            <IconMouse className="h-3 w-3" /> pageviews (5m)
          </p>
        </div>
        {data && data.pages.length > 0 && (
          <div className="min-w-0 flex-1">
            <p className="mb-1.5 flex items-center gap-1 text-[11px] uppercase tracking-wide text-faint">
              <IconBolt className="h-3 w-3" /> Now on
            </p>
            <div className="flex flex-wrap gap-1.5">
              {data.pages.map((p) => (
                <span
                  key={p.key}
                  className="rounded-md bg-raised px-2 py-1 text-xs text-soft"
                  title={`${p.value} pageview${p.value > 1 ? "s" : ""}`}
                >
                  {p.key}
                </span>
              ))}
            </div>
          </div>
        )}
      </div>
      {error && <p className="mt-3 text-xs text-red-400">{error}</p>}
    </section>
  );
}