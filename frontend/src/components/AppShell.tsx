"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { signOut } from "next-auth/react";
import Logo from "@/components/Logo";
import { ThemeToggle } from "@/components/ThemeToggle";
import {
  IconGrid,
  IconChart,
  IconUsers,
  IconLogout,
} from "@/components/icons";

function UserMenu({
  name,
  email,
  role,
}: {
  name: string;
  email: string;
  role: string;
}) {
  const router = useRouter();
  const initial = (name || email || "?").trim().charAt(0).toUpperCase();
  return (
    <div className="flex items-center gap-3 border-t border-edge px-5 py-4">
      <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-indigo-500/15 text-sm font-semibold text-indigo-500">
        {initial}
      </div>
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium text-ink">{name || email}</p>
        <p className="truncate text-xs text-faint">{email}</p>
      </div>
      <button
        title="Sign out"
        onClick={async () => {
          await signOut({ redirect: false });
          router.push("/login");
        }}
        className="rounded-lg p-2 text-faint transition-colors hover:bg-raised hover:text-ink"
      >
        <IconLogout className="h-4 w-4" />
      </button>
    </div>
  );
}

export default function AppShell({
  name,
  email,
  role,
  children,
}: {
  name: string;
  email: string;
  role: string;
  children: React.ReactNode;
}) {
  const pathname = usePathname();

  const nav = [
    { href: "/", label: "Dashboard", icon: IconChart, match: (p: string) => p === "/" },
    { href: "/sites", label: "Sites", icon: IconGrid, match: (p: string) => p.startsWith("/sites") },
    ...(role === "admin"
      ? [{ href: "/admin/users", label: "Users", icon: IconUsers, match: (p: string) => p.startsWith("/admin") }]
      : []),
  ];

  return (
    <div className="flex min-h-screen">
      <aside className="sticky top-0 flex h-screen w-60 shrink-0 flex-col border-r border-edge bg-card">
        <div className="flex items-center justify-between px-5 py-5">
          <Link href="/">
            <Logo size={28} />
          </Link>
          <ThemeToggle className="text-faint" />
        </div>
        <nav className="mt-2 flex-1 space-y-1 px-3">
          {nav.map((n) => {
            const active = n.match(pathname);
            return (
              <Link
                key={n.href}
                href={n.href}
                className={`flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-colors ${
                  active
                    ? "bg-raised font-medium text-ink"
                    : "text-soft hover:bg-raised/60 hover:text-ink"
                }`}
              >
                <n.icon className="h-4 w-4" />
                {n.label}
              </Link>
            );
          })}
        </nav>
        <div className="px-5 py-3">
          <p className="text-[11px] text-faint">
            Created by{" "}
            <a
              href="https://wildandev.tech"
              target="_blank"
              rel="noreferrer"
              className="font-medium text-indigo-500 hover:text-indigo-400"
            >
              WildanDev
            </a>
          </p>
        </div>
        <UserMenu name={name} email={email} role={role} />
      </aside>

      <main className="min-w-0 flex-1">{children}</main>
    </div>
  );
}