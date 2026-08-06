import type { APIRoute } from "astro";
import { proxyBackend } from "@lib/backend";

export const DELETE: APIRoute = async ({ params, request }) => {
  const cookie = request.headers.get("cookie") ?? "";
  return proxyBackend(`/api/instances/${params.id}`, { method: "DELETE", cookie });
};
