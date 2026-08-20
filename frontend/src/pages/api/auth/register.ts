import type { APIRoute } from "astro";
import { proxyBackend } from "@library/backend";

export const POST: APIRoute = async ({ request }) => {
  const body = await request.json();
  return proxyBackend("/api/auth/register", { method: "POST", body });
};
