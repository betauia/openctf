export function afterLogin(url: URL): string {
  const target = url.searchParams.get("redirectTo");
  return target?.startsWith("/") && !target.startsWith("//") && !target.includes("\\") ? target : "/";
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
