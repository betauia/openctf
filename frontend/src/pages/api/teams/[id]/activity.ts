import type { APIRoute } from "astro";
import { proxyBackend } from "@lib/backend";

export const GET: APIRoute = async ({ params }) =>
  proxyBackend(`/api/teams/${params.id}/activity`);
