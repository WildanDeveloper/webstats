import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import { authOptions, apiFetch } from "@/lib/auth";
import type { Site } from "@/lib/types";
import AppShell from "@/components/AppShell";
import SiteList from "@/components/SiteList";

export const dynamic = "force-dynamic";

export default async function SitesPage() {
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
      role={session.user?.role || "user"}
    >
      <div className="mx-auto max-w-4xl px-6 py-8 lg:px-10">
        <header>
          <h1 className="text-2xl font-semibold tracking-tight text-ink">
            Sites
          </h1>
          <p className="mt-1 text-sm text-soft">
            Manage your websites and copy the one-line tracking script.
          </p>
        </header>

        <SiteList initial={sites} token={session.token} />
      </div>
    </AppShell>
  );
}