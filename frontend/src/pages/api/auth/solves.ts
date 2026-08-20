import type { APIRoute } from "astro";
import { proxyBackend } from "@library/backend";

export const GET: APIRoute = async ({ request }) =>
  proxyBackend("/api/auth/solves", { cookie: request.headers.get("cookie") ?? "" });
