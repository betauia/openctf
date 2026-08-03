import type { APIRoute } from "astro";

export const GET: APIRoute = async ({ request }) => {
  const cookie = request.headers.get("cookie") ?? "";
  const res = await fetch("http://backend:8000/api/auth/me", { headers: { cookie } });
  return new Response(await res.text(), {
    status: res.status,
    headers: { "Content-Type": "application/json" },
  });
};
