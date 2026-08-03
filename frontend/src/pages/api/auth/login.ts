import type { APIRoute } from "astro";

export const POST: APIRoute = async ({ request }) => {
  const body = await request.json();
  const res = await fetch("http://backend:8000/api/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  
  const headers = new Headers({ "Content-Type": "application/json" });
  const cookie = res.headers.get("set-cookie");
  if (cookie) headers.set("set-cookie", cookie);
  return new Response(await res.text(), { status: res.status, headers });
};
