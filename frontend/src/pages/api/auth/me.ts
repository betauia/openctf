import type { APIRoute } from "astro";
import { proxyBackend } from "@library/backend";

export const GET: APIRoute = async ({ request }) =>
  proxyBackend("/api/auth/me", { cookie: request.headers.get("cookie") ?? "" });
