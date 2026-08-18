import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import { authOptions, apiFetch } from "@/lib/auth";
import type { Site } from "@/lib/types";
import AppShell from "@/components/AppShell";
import SiteList from "@/components/SiteList";

export const dynamic = "force-dynamic";

export default async function HomePage() {
  const session = await getServerSession(authOptions);
  if (!session?.token) redirect("/login");

  let sites: Site[] = [];
  try {
    sites = await apiFetch<Site[]>("/api/sites", session.token);
  } catch (e) {
    console.error("fetch sites", e);
  }

  return (
    <AppShell
      name={session.user?.name || ""}
      email={session.user?.email || ""}
      token={session.token}
    >
      <div className="mx-auto max-w-4xl px-6 py-8 lg:px-10">
        <header>
          <h1 className="text-2xl font-semibold tracking-tight text-zinc-100">
            Situs kamu
          </h1>
          <p className="mt-1 text-sm text-zinc-400">
            Kelola situs dan salin kode pemasangan untuk mulai mengumpulkan data.
          </p>
        </header>

        <SiteList initial={sites} token={session.token} />
      </div>
    </AppShell>
  );
}