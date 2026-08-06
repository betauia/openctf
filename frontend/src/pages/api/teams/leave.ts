import type { APIRoute } from "astro";
import { proxyBackend } from "@lib/backend";

export const POST: APIRoute = async ({ request }) => {
  const cookie = request.headers.get("cookie") ?? "";
  return proxyBackend("/api/teams/leave", { method: "POST", cookie });
};
