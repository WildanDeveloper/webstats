"use client";

import { useState } from "react";
import { signIn } from "next-auth/react";
import { useRouter } from "next/navigation";
import AuthShell from "@/components/auth/AuthShell";
import { ThemeToggle } from "@/components/ThemeToggle";
import { IconMail, IconLock } from "@/components/icons";

export default function LoginPage() {
  const router = useRouter();
  const [email, setEmail] = useState("admin@webstats.dev");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setLoading(true);
    const res = await signIn("credentials", {
      email,
      password,
      redirect: false,
    });
    setLoading(false);
    if (res?.error) {
      setError("Wrong email or password");
    } else {
      router.push("/");
      router.refresh();
    }
  }

  const inputCls =
    "w-full rounded-lg border border-edge bg-bg py-2.5 pl-10 pr-3 text-sm text-ink placeholder-faint outline-none transition-colors focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500";

  return (
    <AuthShell>
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-semibold tracking-tight text-ink">
            Welcome back
          </h2>
          <p className="mt-1 text-sm text-soft">
            Sign in to your WebStats account.
          </p>
        </div>
        <ThemeToggle />
      </div>

      <form onSubmit={onSubmit} className="mt-7 space-y-4">
        <div>
          <label htmlFor="email" className="text-xs font-medium text-soft">
            Email
          </label>
          <div className="relative mt-1.5">
            <IconMail className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-faint" />
            <input
              id="email"
              type="email"
              required
              autoComplete="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className={inputCls}
              placeholder="name@company.com"
            />
          </div>
        </div>
        <div>
          <label htmlFor="password" className="text-xs font-medium text-soft">
            Password
          </label>
          <div className="relative mt-1.5">
            <IconLock className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-faint" />
            <input
              id="password"
              type="password"
              required
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className={inputCls}
              placeholder="••••••••"
            />
          </div>
        </div>
        {error && (
          <p className="rounded-lg border border-red-900/60 bg-red-950/40 px-3 py-2 text-sm text-red-300">
            {error}
          </p>
        )}
        <button
          type="submit"
          disabled={loading}
          className="flex w-full items-center justify-center gap-2 rounded-lg bg-indigo-600 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
        >
          {loading ? (
            <>
              <svg className="h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none">
                <circle cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="3" className="opacity-25" />
                <path d="M22 12a10 10 0 0 0-10-10" stroke="currentColor" strokeWidth="3" strokeLinecap="round" />
              </svg>
              Signing in...
            </>
          ) : (
            "Sign in"
          )}
        </button>
      </form>

      <div className="mt-6 rounded-lg border border-edge bg-raised/60 px-3.5 py-3 text-xs leading-relaxed text-soft">
        Default admin account:{" "}
        <code className="font-mono text-indigo-500">admin@webstats.dev</code>{" "}
        /{" "}
        <code className="font-mono text-indigo-500">admin123</code>
      </div>

      <p className="mt-6 text-center text-sm text-faint">
        Created by{" "}
        <a
          href="https://wildandev.tech"
          target="_blank"
          rel="noreferrer"
          className="font-medium text-indigo-400 hover:text-indigo-300"
        >
          WildanDev
        </a>
      </p>
    </AuthShell>
  );
}