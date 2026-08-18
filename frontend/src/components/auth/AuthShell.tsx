import {
  IconFeather,
  IconDatabase,
  IconShield,
  IconPulse,
  IconCheck,
} from "@/components/icons";
import Logo from "@/components/Logo";

const FEATURES = [
  { icon: IconFeather, title: "Script super ringan", desc: "2,3kb vanilla JS, tanpa dependency." },
  { icon: IconPulse, title: "Real-time & batch", desc: "Data masuk dalam hitungan detik." },
  { icon: IconDatabase, title: "Data milik kamu", desc: "Tersimpan di PostgreSQL sendiri." },
  { icon: IconShield, title: "Privasi bawaan", desc: "IP di-hash, tanpa cookie pelacak." },
];

export default function AuthShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-screen">
      <aside className="relative hidden w-[46%] flex-col justify-between overflow-hidden border-r border-zinc-800 bg-zinc-900/40 p-10 lg:flex">
        <svg
          className="pointer-events-none absolute inset-0 h-full w-full opacity-[0.07]"
          aria-hidden
        >
          <defs>
            <pattern id="grid" width="32" height="32" patternUnits="userSpaceOnUse">
              <path d="M32 0H0V32" fill="none" stroke="#a1a1aa" strokeWidth="1" />
            </pattern>
          </defs>
          <rect width="100%" height="100%" fill="url(#grid)" />
        </svg>
        <svg
          className="pointer-events-none absolute -right-24 top-1/3 h-[420px] w-[420px] text-indigo-500/10"
          viewBox="0 0 200 200"
          fill="none"
          aria-hidden
        >
          <path
            d="M20 160c30-50 40-80 60-80s30 40 60 20 30-60 40-70"
            stroke="currentColor"
            strokeWidth="3"
            strokeLinecap="round"
          />
          <circle cx="180" cy="30" r="4" fill="currentColor" />
        </svg>

        <div className="relative">
          <Logo />
        </div>

        <div className="relative max-w-md">
          <h1 className="text-3xl font-semibold leading-tight tracking-tight text-zinc-100">
            Analitik web yang ringan, cepat, dan privat.
          </h1>
          <p className="mt-3 text-[15px] leading-relaxed text-zinc-400">
            Pantau pengunjung situs kamu tanpa membebani performa. Satu baris
            script, data tersimpan di database kamu sendiri.
          </p>
          <ul className="mt-8 space-y-5">
            {FEATURES.map((f) => (
              <li key={f.title} className="flex items-start gap-3.5">
                <span className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-zinc-800 bg-zinc-900 text-indigo-400">
                  <f.icon className="h-4 w-4" />
                </span>
                <div>
                  <p className="text-sm font-medium text-zinc-200">{f.title}</p>
                  <p className="text-sm text-zinc-500">{f.desc}</p>
                </div>
              </li>
            ))}
          </ul>
        </div>

        <p className="relative flex items-center gap-2 text-xs text-zinc-500">
          <IconCheck className="h-3.5 w-3.5 text-emerald-500" />
          Open source · self-hosted · tanpa batas pengunjung
        </p>
      </aside>

      <main className="flex flex-1 items-center justify-center p-6">
        <div className="w-full max-w-sm">
          <div className="mb-8 lg:hidden">
            <Logo />
          </div>
          {children}
        </div>
      </main>
    </div>
  );
}