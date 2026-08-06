const KEEP_LOWER = new Set(["pwn", "rev"]);

export function fmt(s: string): string {
  const l = s.toLowerCase();
  return KEEP_LOWER.has(l) ? l : l[0].toUpperCase() + l.slice(1);
}

let _esc: HTMLSpanElement;
export function esc(s: unknown): string {
  _esc ??= document.createElement("span");
  _esc.textContent = String(s);
  return _esc.innerHTML;
}

export function exclusiveActive(selector: string, active: Element) {
  document.querySelectorAll(selector).forEach(el => el.classList.toggle("active", el === active));
}

export function iconFrom(cacheId: string): (name: string) => string {
  return (name) => document.querySelector<HTMLElement>(`#${cacheId} [data-icon="${name}"]`)?.innerHTML ?? "";
}

export function timeAgo(iso: string | null): string {
  if (!iso) return "—";
  const utc = iso.endsWith("Z") || iso.includes("+") ? iso : iso + "Z";
  const mins = Math.floor((Date.now() - new Date(utc).getTime()) / 60000);
  if (mins < 1)  return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24)  return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
}
