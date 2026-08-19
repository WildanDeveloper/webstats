import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import { authOptions, apiFetch } from "@/lib/auth";
import type { FunnelConfig, Invite, Member, Monitor, Site, SiteSettings as SiteSettingsT } from "@/lib/types";
import AppShell from "@/components/AppShell";
import SiteSettings from "@/components/SiteSettings";

export const dynamic = "force-dynamic";

export default async function SiteSettingsPage({
  params,
}: {
  params: { id: string };
}) {
  const session = await getServerSession(authOptions);
  if (!session?.token) redirect("/login");

  let site: Site | null = null;
  let members: Member[] = [];
  let invites: Invite[] = [];
  let settings: SiteSettingsT | null = null;
  let funnels: FunnelConfig[] = [];
  let monitors: Monitor[] = [];
  let error = "";

  try {
    [site, members, invites, settings, funnels, monitors] = await Promise.all([
      apiFetch<Site>(`/api/sites/${params.id}`, session.token),
      apiFetch<Member[]>(`/api/sites/${params.id}/members`, session.token).catch(() => []),
      apiFetch<Invite[]>(`/api/sites/${params.id}/invites`, session.token).catch(() => []),
      apiFetch<SiteSettingsT>(`/api/sites/${params.id}/settings`, session.token).catch(() => null),
      apiFetch<FunnelConfig[]>(`/api/sites/${params.id}/funnels`, session.token).catch(() => []),
      apiFetch<Monitor[]>(`/api/sites/${params.id}/monitors`, session.token).catch(() => []),
    ]);
  } catch (e: any) {
    error = e.message || "Site not found";
  }

  return (
    <AppShell
      name={session.user?.name || ""}
      email={session.user?.email || ""}
      role={session.user?.role || "user"}
    >
      <div className="mx-auto max-w-2xl px-6 py-8 lg:px-10">
        <SiteSettings
          site={site}
          token={session.token}
          loadError={error}
          initialMembers={members}
          initialInvites={invites}
          initialSettings={settings}
          initialFunnels={funnels}
          initialMonitors={monitors}
        />
      </div>
    </AppShell>
  );
}