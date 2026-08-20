import type { APIRoute } from "astro";
import { proxyBackend } from "@library/backend";

export const POST: APIRoute = async () => {
  return proxyBackend("/api/auth/logout", { method: "POST" });
};
