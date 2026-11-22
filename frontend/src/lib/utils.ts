const KEEP_LOWER = new Set(["pwn", "rev"]);

export function fmt(s: string | null | undefined): string {
  if (!s) return "—";
  const l = s.toLowerCase();
  return KEEP_LOWER.has(l) ? l : l[0].toUpperCase() + l.slice(1);
}

export function timeAgo(iso: string | null): string {
  if (!iso) return "—";
  const utc = iso.endsWith("Z") || iso.includes("+") ? iso : iso + "Z";
  const mins = Math.floor((Date.now() - new Date(utc).getTime()) / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
}

export const pad2 = (n: number) => String(n).padStart(2, "0");

export const initial = (name: string) => name[0]?.toUpperCase() ?? "?";
