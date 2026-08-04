import type { APIRoute } from "astro";
import { proxyBackend } from "@lib/backend";

export const POST: APIRoute = async ({ request }) => {
  const body = await request.json();
  return proxyBackend("/api/auth/login", { method: "POST", body });
};
