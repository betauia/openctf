import type { APIRoute } from "astro";

export const POST: APIRoute = async () => {
  const res = await fetch("http://backend:8000/api/auth/logout", { method: "POST" });
  const headers = new Headers({ "Content-Type": "application/json" });
  const cookie = res.headers.get("set-cookie");
  if (cookie) headers.set("set-cookie", cookie);
  return new Response(await res.text(), { status: res.status, headers });
};
