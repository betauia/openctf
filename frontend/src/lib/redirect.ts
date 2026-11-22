export function afterLogin(url: { origin: string; searchParams: { get(name: string): string | null } }): string {
  const target = new URL(url.searchParams.get("redirectTo") ?? "/", url.origin);
  return target.origin === url.origin ? target.pathname + target.search + target.hash : "/";
}

export function loginFor(pathname: string, reason?: string): string {
  const params = new URLSearchParams({ redirectTo: pathname });
  if (reason) params.set("reason", reason);
  return `/login?${params}`;
}

export const SIGN_IN_REASONS: Record<string, string> = {
  "401": "Your session has expired. Please sign in again.",
  "403": "That account is not allowed to view this page.",
};
