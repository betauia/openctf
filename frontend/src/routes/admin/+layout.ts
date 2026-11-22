import { error } from "@sveltejs/kit";
import { requireUser } from "#library/load.js";
import type { LayoutLoad } from "./$types";

export const load: LayoutLoad = async (event) => {
  const data = await requireUser(event);
  if (data.user.role !== "admin") error(404, "Not found");
  return data;
};
