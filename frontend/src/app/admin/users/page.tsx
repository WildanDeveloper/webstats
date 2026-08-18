import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import { authOptions, apiFetch } from "@/lib/auth";
import type { AdminStats, User } from "@/lib/types";
import AppShell from "@/components/AppShell";
import AdminUsers from "@/components/AdminUsers";

export const dynamic = "force-dynamic";

export default async function AdminUsersPage() {
  const session = await getServerSession(authOptions);
  if (!session?.token) redirect("/login");
  if (session.user?.role !== "admin") redirect("/");

  let users: User[] = [];
  let stats: AdminStats | null = null;
  try {
    [users, stats] = await Promise.all([
      apiFetch<User[]>("/api/admin/users", session.token),
      apiFetch<AdminStats>("/api/admin/stats", session.token),
    ]);
  } catch (e) {
    console.error("admin fetch", e);
  }

  return (
    <AppShell
      name={session.user?.name || ""}
      email={session.user?.email || ""}
      role={session.user?.role || "user"}
    >
      <div className="mx-auto max-w-5xl px-6 py-8 lg:px-10">
        <header>
          <h1 className="text-2xl font-semibold tracking-tight text-ink">
            Users
          </h1>
          <p className="mt-1 text-sm text-soft">
            Manage accounts and roles on this WebStats instance.
          </p>
        </header>

        <AdminUsers initial={users} stats={stats} token={session.token} selfId={session.user?.id || ""} />
      </div>
    </AppShell>
  );
}