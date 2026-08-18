import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import { authOptions, apiFetch } from "@/lib/auth";
import type { Site } from "@/lib/types";
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
  let error = "";
  try {
    site = await apiFetch<Site>(`/api/sites/${params.id}`, session.token);
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
        <SiteSettings site={site} token={session.token} loadError={error} />
      </div>
    </AppShell>
  );
}