import { env } from "$env/dynamic/private";
import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

const BACKEND_URL = env.BACKEND_URL ?? "http://backend:8000";

const FORWARD_TO_BACKEND = ["content-type", "accept"];
const FORWARD_TO_BROWSER = ["content-type", "content-disposition"];

function parseCookieValue(setCookie: string): { name: string; value: string } | null {
  const eq = setCookie.indexOf("=");
  if (eq < 0) return null;
  const semi = setCookie.indexOf(";", eq);
  return { name: setCookie.slice(0, eq), value: setCookie.slice(eq + 1, semi < 0 ? undefined : semi) };
}

const proxy: RequestHandler = async ({ request, params, url }) => {
  const headers = new Headers();
  for (const name of FORWARD_TO_BACKEND) {
    const value = request.headers.get(name);
    if (value) headers.set(name, value);
  }

  const session = request.headers.get("x-session");
  const refresh = request.headers.get("x-refresh");
  const parts: string[] = [];
  if (session) parts.push(`session=${session}`);
  if (refresh) parts.push(`refresh=${refresh}`);
  if (parts.length) headers.set("cookie", parts.join("; "));

  let response: Response;
  try {
    response = await fetch(`${BACKEND_URL}/api/${params.path}${url.search}`, {
      method: request.method,
      headers,
      body: request.method === "GET" || request.method === "HEAD" ? undefined : await request.arrayBuffer(),
      redirect: "manual",
    });
  } catch {
    return json({ detail: `Cannot reach the backend at ${BACKEND_URL}` }, { status: 502 });
  }

  const out = new Headers();
  for (const name of FORWARD_TO_BROWSER) {
    const value = response.headers.get(name);
    if (value) out.set(name, value);
  }

  for (const raw of response.headers.getSetCookie()) {
    const parsed = parseCookieValue(raw);
    if (!parsed) continue;
    if (parsed.name === "session") out.set("x-session", parsed.value);
    if (parsed.name === "refresh") out.set("x-refresh", parsed.value);
  }

  return new Response(response.body, { status: response.status, headers: out });
};

export const GET = proxy;
export const POST = proxy;
export const PUT = proxy;
export const PATCH = proxy;
export const DELETE = proxy;
