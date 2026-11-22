import * as api from "#library/api.js";
import { optional, required, requireUser } from "#library/load.js";
import type { LayoutLoad } from "./$types";

export const load: LayoutLoad = async (event) => {
  await requireUser(event);
  const [teams, myTeam] = await Promise.all([required(api.listTeams()), optional(api.myTeam())]);
  return { teams, myTeam };
};
