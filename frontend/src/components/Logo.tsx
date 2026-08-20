import { LogoMark } from "@/components/icons";

export default function Logo({ size = 32 }: { size?: number }) {
  const cls =
    size >= 32 ? "h-8 w-8" : size >= 28 ? "h-7 w-7" : "h-6 w-6";
  return (
    <div className="flex items-center gap-2.5">
      <LogoMark className={`${cls} text-indigo-500`} />
      <span className="text-lg font-semibold tracking-tight text-ink">
        WebStats
      </span>
    </div>
  );
}