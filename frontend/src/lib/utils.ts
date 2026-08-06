const KEEP_LOWER = new Set(["pwn", "rev"]);

export function fmt(s: string): string {
  const l = s.toLowerCase();
  return KEEP_LOWER.has(l) ? l : l[0].toUpperCase() + l.slice(1);
}

export function esc(s: unknown): string {
  return String(s).replace(/[&<>"']/g, (c) => (
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" } as Record<string, string>)[c]
  ));
}
