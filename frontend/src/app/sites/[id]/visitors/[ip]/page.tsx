import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import { authOptions, apiFetch } from "@/lib/auth";
import type { Site } from "@/lib/types";
import AppShell from "@/components/AppShell";
import VisitorDetailView from "@/components/VisitorDetailView";

export const dynamic = "force-dynamic";

export default async function VisitorPage({
  params,
}: {
  params: { id: string; ip: string };
}) {
  const session = await getServerSession(authOptions);
  if (!session?.token) redirect("/login");

  let siteName = "";
  try {
    const site = await apiFetch<Site>(`/api/sites/${params.id}`, session.token);
    siteName = site.name;
  } catch {}

  return (
    <AppShell
      name={session.user?.name || ""}
      email={session.user?.email || ""}
      role={session.user?.role || "user"}
    >
      <VisitorDetailView
        siteId={params.id}
        token={session.token}
        ip={params.ip}
        siteName={siteName}
      />
    </AppShell>
  );
}