"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { signIn } from "next-auth/react";
import { CLIENT_API_URL } from "@/lib/auth";
import { IconShieldCheck } from "@/components/icons";

export default function AcceptInvite({
  email,
  siteName,
  role,
  token,
}: {
  email: string;
  siteName: string;
  role: string;
  token: string;
}) {
  const router = useRouter();
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    if (password.length < 8) {
      setError("Password must be at least 8 characters");
      return;
    }
    if (password !== confirm) {
      setError("Passwords do not match");
      return;
    }
    setBusy(true);
    try {
      const res = await fetch(
        `${CLIENT_API_URL}/api/invites/${token}`,
        { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ password }) },
      );
      if (!res.ok) {
        const j = await res.json().catch(() => ({}));
        throw new Error(j.error || "Failed to accept invite");
      }
      const r = await res.json();
      if (r?.exists) {
        // The email already had an account — its password was NOT changed.
        setError("This email already has an account. Sign in to access the site.");
        setTimeout(() => router.push("/login"), 1500);
        return;
      }
      const s = await signIn("credentials", {
        email,
        password,
        redirect: false,
      });
      if (s?.error) {
        setError("Account created. Please sign in with your new password.");
        router.push("/login");
        return;
      }
      router.push("/");
    } catch (err: any) {
      setError(err.message);
    }
    setBusy(false);
  }

  return (
    <div className="mx-auto mt-16 w-full max-w-md px-6">
      <div className="rounded-xl border border-edge bg-card p-8">
        <span className="flex h-12 w-12 items-center justify-center rounded-xl bg-emerald-500/10 text-emerald-500">
          <IconShieldCheck className="h-6 w-6" />
        </span>
        <h1 className="mt-4 text-xl font-semibold text-ink">Join {siteName}</h1>
        <p className="mt-1 text-sm text-faint">
          You have been invited as a {role}. Create a password for {email} to get started.
        </p>
        <form onSubmit={submit} className="mt-6 space-y-4">
          <div>
            <label className="text-xs font-medium text-soft">Password</label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="mt-1.5 w-full rounded-lg border border-edge bg-bg px-3 py-2.5 text-sm text-ink outline-none focus:border-indigo-500"
              required
            />
          </div>
          <div>
            <label className="text-xs font-medium text-soft">Confirm password</label>
            <input
              type="password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              className="mt-1.5 w-full rounded-lg border border-edge bg-bg px-3 py-2.5 text-sm text-ink outline-none focus:border-indigo-500"
              required
            />
          </div>
          {error && <p className="text-sm text-red-400">{error}</p>}
          <button
            type="submit"
            disabled={busy}
            className="w-full rounded-lg bg-indigo-600 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-indigo-500 disabled:opacity-60"
          >
            {busy ? "Creating account..." : "Accept invite"}
          </button>
        </form>
      </div>
    </div>
  );
}