import type { APIRoute } from "astro";
import { proxyBackend } from "@library/backend";

export const GET: APIRoute = async ({ params }) =>
  proxyBackend(`/api/challenges/${params.id}/solves`);
