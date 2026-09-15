"use client";

import { useRouter } from "next/navigation";
import { signOut, useSession } from "next-auth/react";
import { CLIENT_API_URL } from "@/lib/auth";

export function LogoutButton() {
  const router = useRouter();
  const { data: session } = useSession();
  return (
    <button
      onClick={async () => {
        try {
          await fetch(`${CLIENT_API_URL}/api/auth/logout`, {
            method: "POST",
            headers: session?.token ? { Authorization: `Bearer ${session.token}` } : {},
          });
        } catch {}
        await signOut({ redirect: false });
        router.push("/login");
      }}
      className="rounded-lg border border-zinc-700 px-3 py-1.5 text-sm text-zinc-300 hover:bg-zinc-800"
    >
      Sign out
    </button>
  );
}