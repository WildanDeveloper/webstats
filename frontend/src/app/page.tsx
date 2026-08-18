import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import { authOptions, apiFetch } from "@/lib/auth";
import type { RootOverview, Site } from "@/lib/types";
import AppShell from "@/components/AppShell";
import DashboardView from "@/components/DashboardView";

export const dynamic = "force-dynamic";

export default async function DashboardPage({
  searchParams,
}: {
  searchParams: { period?: string };
}) {
  const session = await getServerSession(authOptions);
  if (!session?.token) redirect("/login");

  const period = searchParams.period || "7d";
  const q = `period=${period}`;

  let overview: RootOverview | null = null;
  let sites: Site[] = [];
  try {
    [overview, sites] = await Promise.all([
      apiFetch<RootOverview>(`/api/overview?${q}`, session.token),
      apiFetch<Site[]>("/api/sites", session.token),
    ]);
  } catch (e) {
    console.error("dashboard fetch", e);
  }

  return (
    <AppShell
      name={session.user?.name || ""}
      email={session.user?.email || ""}
      role={session.user?.role || "user"}
    >
      <div className="mx-auto max-w-6xl px-6 py-8 lg:px-10">
        <DashboardView
          period={period}
          overview={overview}
          sites={sites}
          userName={session.user?.name || "there"}
        />
      </div>
    </AppShell>
  );
}