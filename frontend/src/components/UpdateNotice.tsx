"use client";

import { useEffect, useState } from "react";
import { CLIENT_API_URL } from "@/lib/auth";

const GITHUB_REPO =
  process.env.NEXT_PUBLIC_GITHUB_REPO || "WildanDeveloper/webstats";

function parseSemver(v: string): [number, number, number] | null {
  const m = v.replace(/^v/, "").trim().match(/^(\d+)\.(\d+)\.(\d+)/);
  return m ? [+m[1], +m[2], +m[3]] : null;
}

/** true when candidate a is strictly newer than installed b */
export function isNewer(a: string, b: string): boolean {
  const x = parseSemver(a);
  const y = parseSemver(b);
  if (!x || !y) return false;
  if (x[0] !== y[0]) return x[0] > y[0];
  if (x[1] !== y[1]) return x[1] > y[1];
  return x[2] > y[2];
}

type Release = { tag: string; url: string };

/**
 * Reads the version of the running backend — the single source of truth for
 * what is installed.
 */
export function useInstalledVersion(): string {
  const [version, setVersion] = useState("");
  useEffect(() => {
    let active = true;
    fetch(`${CLIENT_API_URL}/api/version`)
      .then((r) => (r.ok ? r.json() : null))
      .then((d) => {
        if (active && d?.version) setVersion(String(d.version));
      })
      .catch(() => {});
    return () => {
      active = false;
    };
  }, []);
  return version;
}

/**
 * Polls the latest GitHub release (cached 6h in localStorage) and reports it
 * when it is newer than the installed version.
 */
function useLatestRelease(installed: string): Release | null {
  const [release, setRelease] = useState<Release | null>(null);

  useEffect(() => {
    if (!installed) return;
    try {
      const cached = JSON.parse(
        localStorage.getItem("wst_release_check") || "null",
      );
      if (cached?.tag && Date.now() - cached.at < 6 * 3600 * 1000) {
        if (isNewer(cached.tag, installed))
          setRelease({ tag: cached.tag, url: cached.url });
        return;
      }
    } catch {}

    let active = true;
    fetch(`https://api.github.com/repos/${GITHUB_REPO}/releases/latest`)
      .then((r) => (r.ok ? r.json() : null))
      .then((d) => {
        if (!active || !d?.tag_name) return;
        const tag = String(d.tag_name).replace(/^v/, "");
        const entry = { at: Date.now(), tag, url: d.html_url };
        try {
          localStorage.setItem("wst_release_check", JSON.stringify(entry));
        } catch {}
        if (isNewer(tag, installed)) setRelease({ tag, url: entry.url });
      })
      .catch(() => {});
    return () => {
      active = false;
    };
  }, [installed]);

  return release;
}

/**
 * Banner shown in the sidebar when a newer WebStats release is available.
 * Dismissal is remembered per released version.
 */
export default function UpdateNotice({ installed }: { installed: string }) {
  const latest = useLatestRelease(installed);
  const [dismissedTag, setDismissedTag] = useState("");

  useEffect(() => {
    setDismissedTag(localStorage.getItem("wst_update_dismissed") || "");
  }, []);

  if (!latest || dismissedTag === latest.tag) return null;

  function dismiss() {
    if (!latest) return;
    try {
      localStorage.setItem("wst_update_dismissed", latest.tag);
    } catch {}
    setDismissedTag(latest.tag);
  }

  return (
    <div className="mx-3 mb-2 rounded-lg border border-indigo-500/30 bg-indigo-500/10 px-3 py-2">
      <p className="text-xs font-medium text-indigo-400">Update available</p>
      <p className="mt-0.5 text-[11px] text-faint">
        v{latest.tag} · you are on v{installed}
      </p>
      <div className="mt-1.5 flex items-center gap-3 text-[11px]">
        <a
          href={latest.url}
          target="_blank"
          rel="noreferrer"
          className="font-medium text-indigo-400 hover:text-indigo-300"
        >
          View release
        </a>
        <button
          onClick={dismiss}
          className="ml-auto text-faint transition-colors hover:text-ink"
          title="Dismiss"
        >
          Dismiss
        </button>
      </div>
    </div>
  );
}
