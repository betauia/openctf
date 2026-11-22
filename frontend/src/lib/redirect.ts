/**
 * Where to send a user after signing in. Only same-site paths are accepted, so a
 * crafted ?redirectTo= cannot bounce someone to another host.
 */
export function afterLogin(url: URL): string {
  const target = url.searchParams.get("redirectTo");
  return target?.startsWith("/") && !target.startsWith("//") ? target : "/";
}

/**
 * Login URL that returns the user to the page they were trying to reach, and
 * says why they were sent there, so an unexpected bounce is never silent.
 */
export function loginFor(pathname: string, reason?: string): string {
  const params = new URLSearchParams({ redirectTo: pathname });
  if (reason) params.set("reason", reason);
  return `/login?${params}`;
}

export const SIGN_IN_REASONS: Record<string, string> = {
  "401": "Your session has expired. Please sign in again.",
  "403": "That account is not allowed to view this page.",
};
