const BACKEND_URL = "http://backend:8000";

export async function proxyBackend(
  path: string,
  opts: { method?: string; body?: unknown; cookie?: string } = {},
): Promise<Response> {
  const reqHeaders: Record<string, string> = {};
  if (opts.body !== undefined) reqHeaders["Content-Type"] = "application/json";
  if (opts.cookie) reqHeaders["cookie"] = opts.cookie;

  const res = await fetch(`${BACKEND_URL}${path}`, {
    method: opts.method ?? "GET",
    headers: reqHeaders,
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
  });

  const headers = new Headers({ "Content-Type": "application/json" });
  const setCookie = res.headers.get("set-cookie");
  if (setCookie) headers.set("set-cookie", setCookie);
  return new Response(await res.text(), { status: res.status, headers });
}
