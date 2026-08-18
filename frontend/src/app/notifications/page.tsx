import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import { authOptions, apiFetch } from "@/lib/auth";
import type { NotifLog, NotifProvider, NotifRule, Report, Site } from "@/lib/types";
import AppShell from "@/components/AppShell";
import NotificationsView from "@/components/NotificationsView";

export const dynamic = "force-dynamic";

export default async function NotificationsPage() {
  const session = await getServerSession(authOptions);
  if (!session?.token) redirect("/login");

  let providers: NotifProvider[] = [];
  let rules: NotifRule[] = [];
  let logs: NotifLog[] = [];
  let reports: Report[] = [];
  let sites: Site[] = [];

  try {
    [providers, rules, logs, reports, sites] = await Promise.all([
      apiFetch<NotifProvider[]>("/api/notifications/providers", session.token),
      apiFetch<NotifRule[]>("/api/notifications/rules", session.token),
      apiFetch<NotifLog[]>("/api/notifications/logs", session.token),
      apiFetch<Report[]>("/api/notifications/reports", session.token),
      apiFetch<Site[]>("/api/sites", session.token),
    ]);
  } catch {}

  return (
    <AppShell
      name={session.user?.name || ""}
      email={session.user?.email || ""}
      role={session.user?.role || "user"}
    >
      <div className="mx-auto max-w-6xl px-6 py-8 lg:px-10">
        <header>
          <h1 className="text-2xl font-bold text-ink">Notifications</h1>
          <p className="mt-1 text-sm text-faint">
            Get alerts by email or webhook when a site goes down, comes back, or traffic spikes.
          </p>
        </header>
        <NotificationsView
          providers={providers}
          rules={rules}
          logs={logs}
          reports={reports}
          sites={sites}
          token={session.token}
        />
      </div>
    </AppShell>
  );
}