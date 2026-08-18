"use client";

import { useState } from "react";
import { apiFetch } from "@/lib/auth";
import type { AdminStats, User } from "@/lib/types";
import {
  IconPlus,
  IconTrash,
  IconLock,
  IconUsers,
  IconGrid,
  IconMouse,
  IconPulse,
} from "@/components/icons";

const nf = new Intl.NumberFormat("en-US");

export default function AdminUsers({
  initial,
  stats,
  token,
  selfId,
}: {
  initial: User[];
  stats: AdminStats | null;
  token: string;
  selfId: string;
}) {
  const [users, setUsers] = useState<User[]>(initial);
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [role, setRole] = useState("user");
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState("");
  const [actionMsg, setActionMsg] = useState("");

  async function addUser(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setActionMsg("");
    setCreating(true);
    try {
      const u = await apiFetch<User>("/api/admin/users", token, {
        method: "POST",
        body: JSON.stringify({ name, email, password, role }),
      });
      setUsers((prev) => [u, ...prev]);
      setName("");
      setEmail("");
      setPassword("");
      setRole("user");
    } catch (err: any) {
      setError(err.message);
    }
    setCreating(false);
  }

  async function changeRole(id: string, newRole: string) {
    setActionMsg("");
    try {
      await apiFetch(`/api/admin/users/${id}`, token, {
        method: "PATCH",
        body: JSON.stringify({ role: newRole }),
      });
      setUsers((prev) =>
        prev.map((u) => (u.id === id ? { ...u, role: newRole } : u)),
      );
    } catch (err: any) {
      setActionMsg(err.message);
    }
  }

  async function resetPassword(id: string, currentName: string) {
    const pw = prompt(`New password for "${currentName}" (min 8 chars)`);
    if (!pw) return;
    setActionMsg("");
    try {
      await apiFetch(`/api/admin/users/${id}`, token, {
        method: "PATCH",
        body: JSON.stringify({ password: pw }),
      });
      setActionMsg("Password updated");
    } catch (err: any) {
      setActionMsg(err.message);
    }
  }

  async function remove(id: string, currentName: string) {
    if (!confirm(`Delete user "${currentName}"? Their sites and data are removed too.`)) {
      return;
    }
    setActionMsg("");
    try {
      await apiFetch(`/api/admin/users/${id}`, token, { method: "DELETE" });
      setUsers((prev) => prev.filter((u) => u.id !== id));
    } catch (err: any) {
      setActionMsg(err.message);
    }
  }

  const statCards = stats
    ? [
        { label: "Users", value: nf.format(stats.users), icon: IconUsers, tint: "text-indigo-500 bg-indigo-500/10" },
        { label: "Sites", value: nf.format(stats.sites), icon: IconGrid, tint: "text-sky-500 bg-sky-500/10" },
        { label: "Pageviews", value: nf.format(stats.pageviews), icon: IconMouse, tint: "text-emerald-500 bg-emerald-500/10" },
        { label: "Events", value: nf.format(stats.events), icon: IconPulse, tint: "text-amber-500 bg-amber-500/10" },
      ]
    : [];

  const inputCls =
    "w-full rounded-lg border border-edge bg-bg px-3 py-2 text-sm text-ink placeholder-faint outline-none transition-colors focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500";

  return (
    <div className="mt-8 space-y-6">
      {statCards.length > 0 && (
        <div className="grid grid-cols-2 gap-4 xl:grid-cols-4">
          {statCards.map((c) => (
            <div key={c.label} className="rounded-xl border border-edge bg-card p-5">
              <span className={`flex h-9 w-9 items-center justify-center rounded-lg ${c.tint}`}>
                <c.icon className="h-[18px] w-[18px]" />
              </span>
              <p className="mt-3 text-2xl font-semibold tracking-tight text-ink">
                {c.value}
              </p>
              <p className="mt-0.5 text-xs text-faint">{c.label}</p>
            </div>
          ))}
        </div>
      )}

      <form
        onSubmit={addUser}
        className="flex flex-wrap items-end gap-3 rounded-xl border border-edge bg-card p-5"
      >
        <div className="w-40">
          <label className="text-xs font-medium text-soft">Name</label>
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            className={`${inputCls} mt-1.5`}
            placeholder="John Doe"
          />
        </div>
        <div className="w-44">
          <label className="text-xs font-medium text-soft">Email</label>
          <input
            type="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className={`${inputCls} mt-1.5`}
            placeholder="user@company.com"
          />
        </div>
        <div className="w-40">
          <label className="text-xs font-medium text-soft">Password</label>
          <input
            type="password"
            required
            minLength={8}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className={`${inputCls} mt-1.5`}
            placeholder="min 8 chars"
          />
        </div>
        <div className="w-28">
          <label className="text-xs font-medium text-soft">Role</label>
          <select
            value={role}
            onChange={(e) => setRole(e.target.value)}
            className={`${inputCls} mt-1.5`}
          >
            <option value="user">user</option>
            <option value="admin">admin</option>
          </select>
        </div>
        <button
          type="submit"
          disabled={creating}
          className="flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-indigo-500 disabled:opacity-60"
        >
          <IconPlus className="h-4 w-4" />
          {creating ? "Creating..." : "Add user"}
        </button>
        {error && <p className="w-full text-sm text-red-400">{error}</p>}
      </form>

      {actionMsg && (
        <p className="rounded-lg border border-edge bg-card px-4 py-2.5 text-sm text-soft">
          {actionMsg}
        </p>
      )}

      <div className="overflow-x-auto rounded-xl border border-edge bg-card">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-edge text-left text-xs uppercase tracking-wide text-faint">
              <th className="px-5 py-3 font-medium">User</th>
              <th className="px-5 py-3 font-medium">Role</th>
              <th className="px-5 py-3 font-medium">Joined</th>
              <th className="px-5 py-3 text-right font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            {users.map((u) => (
              <tr key={u.id} className="border-b border-edge last:border-0">
                <td className="px-5 py-3">
                  <div className="flex items-center gap-3">
                    <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-indigo-500/15 text-xs font-semibold text-indigo-500">
                      {(u.name || u.email || "?").charAt(0).toUpperCase()}
                    </div>
                    <div>
                      <p className="font-medium text-ink">
                        {u.name || "—"}
                        {u.id === selfId && (
                          <span className="ml-2 rounded-full border border-edge px-2 py-0.5 text-[10px] text-faint">
                            you
                          </span>
                        )}
                      </p>
                      <p className="text-xs text-faint">{u.email}</p>
                    </div>
                  </div>
                </td>
                <td className="px-5 py-3">
                  <span
                    className={`rounded-full px-2.5 py-1 text-[11px] font-medium ${
                      u.role === "admin"
                        ? "bg-indigo-500/15 text-indigo-500"
                        : "bg-raised text-faint"
                    }`}
                  >
                    {u.role}
                  </span>
                </td>
                <td className="px-5 py-3 text-faint">
                  {new Date(u.created_at).toLocaleDateString("en-US", {
                    year: "numeric",
                    month: "short",
                    day: "numeric",
                  })}
                </td>
                <td className="px-5 py-3">
                  <div className="flex items-center justify-end gap-3">
                    <select
                      value={u.role}
                      onChange={(e) => changeRole(u.id, e.target.value)}
                      className="rounded-md border border-edge bg-bg px-2 py-1 text-xs text-soft outline-none focus:border-indigo-500"
                    >
                      <option value="user">user</option>
                      <option value="admin">admin</option>
                    </select>
                    <button
                      onClick={() => resetPassword(u.id, u.name || u.email)}
                      title="Reset password"
                      className="rounded-md p-1.5 text-faint transition-colors hover:bg-raised hover:text-ink"
                    >
                      <IconLock className="h-4 w-4" />
                    </button>
                    <button
                      onClick={() => remove(u.id, u.name || u.email)}
                      title="Delete user"
                      disabled={u.id === selfId}
                      className="rounded-md p-1.5 text-faint transition-colors hover:bg-red-950/40 hover:text-red-400 disabled:cursor-not-allowed disabled:opacity-30"
                    >
                      <IconTrash className="h-4 w-4" />
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}