import type { APIRoute } from "astro";
import { proxyBackend } from "@library/backend";

export const POST: APIRoute = async ({ request }) => {
  const body = await request.json();
  const cookie = request.headers.get("cookie") ?? "";
  return proxyBackend("/api/teams", { method: "POST", body, cookie });
};
