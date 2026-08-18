"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { signOut } from "next-auth/react";
import Logo from "@/components/Logo";
import { IconGrid, IconCode, IconLogout, IconExternal } from "@/components/icons";

function UserMenu({
  name,
  email,
  token,
}: {
  name: string;
  email: string;
  token: string;
}) {
  const router = useRouter();
  const initial = (name || email || "?").trim().charAt(0).toUpperCase();
  return (
    <div className="flex items-center gap-3 border-t border-zinc-800 px-5 py-4">
      <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-indigo-500/15 text-sm font-semibold text-indigo-300">
        {initial}
      </div>
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium text-zinc-200">{name || email}</p>
        <p className="truncate text-xs text-zinc-500">{email}</p>
      </div>
      <button
        title="Keluar"
        onClick={async () => {
          await signOut({ redirect: false });
          router.push("/login");
        }}
        className="rounded-lg p-2 text-zinc-500 transition-colors hover:bg-zinc-800 hover:text-zinc-200"
      >
        <IconLogout className="h-4 w-4" />
      </button>
    </div>
  );
}

export default function AppShell({
  name,
  email,
  token,
  children,
}: {
  name: string;
  email: string;
  token: string;
  children: React.ReactNode;
}) {
  const pathname = usePathname();

  const nav = [
    { href: "/", label: "Situs", icon: IconGrid },
    { href: "/#install", label: "Cara pasang", icon: IconCode },
  ];

  return (
    <div className="flex min-h-screen">
      <aside className="sticky top-0 flex h-screen w-60 shrink-0 flex-col border-r border-zinc-800 bg-zinc-900/40">
        <div className="px-5 py-5">
          <Link href="/">
            <Logo size={28} />
          </Link>
        </div>
        <nav className="mt-2 flex-1 space-y-1 px-3">
          {nav.map((n) => {
            const active =
              n.href === "/" ? pathname === "/" : pathname.startsWith("/sites");
            return (
              <Link
                key={n.href}
                href={n.href}
                className={`flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-colors ${
                  active
                    ? "bg-zinc-800/80 font-medium text-zinc-100"
                    : "text-zinc-400 hover:bg-zinc-800/50 hover:text-zinc-200"
                }`}
              >
                <n.icon className="h-4 w-4" />
                {n.label}
              </Link>
            );
          })}
          <a
            href="https://github.com"
            target="_blank"
            rel="noreferrer"
            className="flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-zinc-400 transition-colors hover:bg-zinc-800/50 hover:text-zinc-200"
          >
            <IconExternal className="h-4 w-4" />
            Dokumentasi
          </a>
        </nav>
        <UserMenu name={name} email={email} token={token} />
      </aside>

      <main className="min-w-0 flex-1">{children}</main>
    </div>
  );
}