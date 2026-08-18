import type { APIRoute } from "astro";
import { BACKEND_URL } from "@library/backend";

export const GET: APIRoute = async ({ params }) => {
  const res = await fetch(`${BACKEND_URL}/api/challenges/${params.id}/files/${params.filename}`);
  if (!res.ok) return new Response("Not found", { status: 404 });
  return new Response(res.body, {
    headers: {
      "Content-Type": res.headers.get("Content-Type") ?? "application/octet-stream",
      "Content-Disposition": res.headers.get("Content-Disposition") ?? "",
    },
  });
};
