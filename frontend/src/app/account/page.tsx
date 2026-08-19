import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import { authOptions } from "@/lib/auth";
import AppShell from "@/components/AppShell";
import AccountView from "@/components/AccountView";

export const dynamic = "force-dynamic";

export default async function AccountPage() {
  const session = await getServerSession(authOptions);
  if (!session?.token) redirect("/login");

  return (
    <AppShell
      name={session.user?.name || ""}
      email={session.user?.email || ""}
      role={session.user?.role || "user"}
    >
      <div className="mx-auto max-w-2xl px-6 py-8 lg:px-10">
        <AccountView token={session.token} email={session.user?.email || ""} />
      </div>
    </AppShell>
  );
}