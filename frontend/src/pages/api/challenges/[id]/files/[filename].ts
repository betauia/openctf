import type { APIRoute } from "astro";

export const GET: APIRoute = async ({ params }) => {
  const res = await fetch(`http://backend:8000/api/challenges/${params.id}/files/${params.filename}`);
  if (!res.ok) return new Response("Not found", { status: 404 });
  return new Response(res.body, {
    headers: {
      "Content-Type": res.headers.get("Content-Type") ?? "application/octet-stream",
      "Content-Disposition": res.headers.get("Content-Disposition") ?? "",
    },
  });
};
