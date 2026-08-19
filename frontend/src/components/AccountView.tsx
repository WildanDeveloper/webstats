"use client";

import { useState } from "react";
import { apiFetch } from "@/lib/auth";
import type { ApiKey } from "@/lib/types";
import {
  IconCheck,
  IconCopy,
  IconKey,
  IconLock,
  IconPlus,
  IconTrash,
} from "@/components/icons";

export default function AccountView({
  token,
  email,
}: {
  token: string;
  email: string;
}) {
  const [cur, setCur] = useState("");
  const [next, setNext] = useState("");
  const [pwMsg, setPwMsg] = useState("");
  const [pwErr, setPwErr] = useState("");
  const [saving, setSaving] = useState(false);

  const [keys, setKeys] = useState<ApiKey[]>([]);
  const [keyName, setKeyName] = useState("");
  const [newKey, setNewKey] = useState("");
  const [keyMsg, setKeyMsg] = useState("");

  async function loadKeys() {
    try {
      const res = await apiFetch<ApiKey[]>(`/api/account/api-keys`, token);
      setKeys(res);
    } catch {
      setKeys([]);
    }
  }

  async function changePassword(e: React.FormEvent) {
    e.preventDefault();
    setPwMsg("");
    setPwErr("");
    setSaving(true);
    try {
      await apiFetch(`/api/account/password`, token, {
        method: "POST",
        body: JSON.stringify({ current: cur, new: next }),
      });
      setPwMsg("Password updated");
      setCur("");
      setNext("");
    } catch (err: any) {
      setPwErr(err.message);
    }
    setSaving(false);
  }

  async function createKey(e: React.FormEvent) {
    e.preventDefault();
    setKeyMsg("");
    setNewKey("");
    try {
      const res = await apiFetch<{ key: string; name: string }>(`/api/account/api-keys`, token, {
        method: "POST",
        body: JSON.stringify({ name: keyName }),
      });
      setNewKey(res.key);
      setKeyName("");
      loadKeys();
    } catch (err: any) {
      setKeyMsg(err.message);
    }
  }

  async function revoke(id: string) {
    if (!confirm("Revoke this API key?")) return;
    try {
      await apiFetch(`/api/account/api-keys/${id}`, token, { method: "DELETE" });
      setKeys((prev) => prev.filter((k) => k.id !== id));
    } catch (err: any) {
      setKeyMsg(err.message);
    }
  }

  const inputCls =
    "w-full rounded-lg border border-edge bg-bg px-3 py-2 text-sm text-ink placeholder-faint outline-none focus:border-indigo-500";

  return (
    <div className="space-y-6">
      <header>
        <h1 className="text-2xl font-semibold tracking-tight text-ink">Account</h1>
        <p className="mt-0.5 text-sm text-faint">{email}</p>
      </header>

      <section className="rounded-xl border border-edge bg-card p-6">
        <h2 className="flex items-center gap-2 text-sm font-semibold text-ink">
          <IconLock className="h-4 w-4 text-indigo-500" />
          Change password
        </h2>
        <form onSubmit={changePassword} className="mt-4 space-y-4">
          <div>
            <label className="text-xs font-medium text-soft">Current password</label>
            <input
              type="password"
              value={cur}
              onChange={(e) => setCur(e.target.value)}
              className={`${inputCls} mt-1.5`}
              required
            />
          </div>
          <div>
            <label className="text-xs font-medium text-soft">New password</label>
            <input
              type="password"
              value={next}
              onChange={(e) => setNext(e.target.value)}
              className={`${inputCls} mt-1.5`}
              required
              minLength={8}
            />
          </div>
          {pwMsg && (
            <p className="flex items-center gap-1.5 text-sm text-emerald-500">
              <IconCheck className="h-4 w-4" /> {pwMsg}
            </p>
          )}
          {pwErr && <p className="text-sm text-red-400">{pwErr}</p>}
          <button
            type="submit"
            disabled={saving}
            className="rounded-lg bg-indigo-600 px-4 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-indigo-500 disabled:opacity-60"
          >
            {saving ? "Saving..." : "Update password"}
          </button>
        </form>
      </section>

      <section className="rounded-xl border border-edge bg-card p-6">
        <h2 className="flex items-center gap-2 text-sm font-semibold text-ink">
          <IconKey className="h-4 w-4 text-indigo-500" />
          API keys
        </h2>
        <p className="mt-0.5 text-xs text-faint">
          Keys grant full access to your sites. Use them as a Bearer token in API requests.
        </p>
        <form onSubmit={createKey} className="mt-4 flex flex-wrap items-center gap-2">
          <input
            value={keyName}
            onChange={(e) => setKeyName(e.target.value)}
            placeholder="Key name, e.g. CI"
            className="min-w-52 flex-1 rounded-lg border border-edge bg-bg px-3 py-2 text-sm text-ink placeholder-faint outline-none focus:border-indigo-500"
          />
          <button
            type="submit"
            className="flex items-center gap-1.5 rounded-lg bg-indigo-600 px-3.5 py-2 text-sm font-medium text-white transition-colors hover:bg-indigo-500"
          >
            <IconPlus className="h-4 w-4" /> Create key
          </button>
        </form>

        {keyMsg && <p className="mt-3 text-sm text-red-400">{keyMsg}</p>}
        {newKey && (
          <div className="mt-3 rounded-lg border border-indigo-500/30 bg-indigo-500/10 p-3">
            <p className="text-xs font-medium text-indigo-400">
              Copy your key now. You will not see it again:
            </p>
            <div className="mt-1.5 flex items-center gap-2">
              <code className="min-w-0 flex-1 truncate rounded-md border border-edge bg-bg px-2.5 py-2 font-mono text-xs text-soft">
                {newKey}
              </code>
              <button
                onClick={() => {
                  navigator.clipboard.writeText(newKey);
                  setKeyMsg("Key copied");
                }}
                className="rounded-md border border-edge px-2 py-1 text-xs text-soft transition-colors hover:bg-raised"
              >
                <IconCopy className="h-3.5 w-3.5" />
              </button>
            </div>
          </div>
        )}

        <div className="mt-4 space-y-2">
          {keys.length === 0 && <p className="text-xs text-faint">No API keys yet.</p>}
          {keys.map((k) => (
            <div key={k.id} className="flex items-center gap-3 rounded-lg border border-edge/60 px-3 py-2.5">
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium text-ink">{k.name}</p>
                <p className="truncate text-xs text-faint">
                  {k.prefix}… · {k.last_used_at
                    ? `last used ${new Date(k.last_used_at).toLocaleDateString("en-US", { month: "short", day: "numeric" })}`
                    : "never used"}
                </p>
              </div>
              <button
                onClick={() => revoke(k.id)}
                className="rounded-lg p-2 text-faint transition-colors hover:bg-raised hover:text-red-500"
                title="Revoke key"
              >
                <IconTrash className="h-4 w-4" />
              </button>
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}