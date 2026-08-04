import type { APIRoute } from "astro";
import { proxyBackend } from "@lib/backend";

export const POST: APIRoute = async () => proxyBackend("/api/auth/logout", { method: "POST" });
