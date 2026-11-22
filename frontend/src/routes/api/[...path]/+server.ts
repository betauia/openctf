import { error, json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

const BACKEND_URL = process.env.BACKEND_URL ?? "http://backend:8000";

const FORWARD_TO_BACKEND = ["content-type", "accept"];
const FORWARD_TO_BROWSER = ["content-type", "content-disposition"];
const AUTH_COOKIES = ["session", "refresh"];

const proxy: RequestHandler = async ({ request, params, url, cookies }) => {
  const target = new URL(`${BACKEND_URL}/api/${params.path}${url.search}`);
  if (!target.pathname.startsWith("/api/")) error(404, "Not found");

  const headers = new Headers();
  for (const name of FORWARD_TO_BACKEND) {
    const value = request.headers.get(name);
    if (value) headers.set(name, value);
  }

  const parts = AUTH_COOKIES.flatMap((name) => {
    const value = cookies.get(name);
    return value ? [`${name}=${value}`] : [];
  });
  if (parts.length) headers.set("cookie", parts.join("; "));

  let response: Response;
  try {
    response = await fetch(target, {
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
    if (AUTH_COOKIES.includes(raw.slice(0, raw.indexOf("=")))) {
      out.append("set-cookie", url.protocol === "https:" ? `${raw}; Secure` : raw);
    }
  }

  return new Response(response.body, { status: response.status, headers: out });
};

export const GET = proxy;
export const POST = proxy;
export const PUT = proxy;
export const PATCH = proxy;
export const DELETE = proxy;
