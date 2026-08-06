const KEEP_LOWER = new Set(["pwn", "rev"]);

export function fmt(s: string): string {
  const l = s.toLowerCase();
  return KEEP_LOWER.has(l) ? l : l[0].toUpperCase() + l.slice(1);
}

const _esc = document.createElement("span");
export function esc(s: unknown): string {
  _esc.textContent = String(s);
  return _esc.innerHTML;
}

export function exclusiveActive(selector: string, active: Element) {
  document.querySelectorAll(selector).forEach(el => el.classList.toggle("active", el === active));
}
