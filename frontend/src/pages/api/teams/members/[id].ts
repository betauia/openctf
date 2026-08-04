import type { APIRoute } from "astro";
import { proxyBackend } from "@lib/backend";

export const GET: APIRoute = async ({ request, params }) =>
  proxyBackend(`/api/teams/members/${params.id}`, { cookie: request.headers.get("cookie") ?? "" });
