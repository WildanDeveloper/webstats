import AppShell from "@/components/AppShell";
import AcceptInvite from "@/components/AcceptInvite";

export const dynamic = "force-dynamic";

export default async function InvitePage({
  params,
}: {
  params: { token: string };
}) {
  const base = process.env.NEXT_PUBLIC_API_URL || "http://127.0.0.1:8086";
  let info: { email: string; site_name: string; role: string } | null = null;
  let error = "";

  try {
    const res = await fetch(`${base}/api/invites/${params.token}`, { cache: "no-store" });
    if (!res.ok) throw new Error("invite not found");
    info = await res.json();
  } catch {
    error = "This invite is invalid or has expired.";
  }

  return (
    <AppShell name="" email="" role="">
      {info ? (
        <AcceptInvite
          email={info.email}
          siteName={info.site_name}
          role={info.role}
          token={params.token}
        />
      ) : (
        <div className="mx-auto mt-16 w-full max-w-md px-6">
          <div className="rounded-xl border border-red-900/60 bg-red-950/40 p-8 text-center">
            <h1 className="text-lg font-semibold text-red-300">Invite not found</h1>
            <p className="mt-2 text-sm text-red-300/80">{error}</p>
            <p className="mt-1 text-sm text-red-300/80">
              Ask the site owner for a new invite link.
            </p>
          </div>
        </div>
      )}
    </AppShell>
  );
}