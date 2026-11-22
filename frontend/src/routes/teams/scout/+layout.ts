import * as api from "@library/api";
import { optional, required, requireUser } from "@library/load";
import type { LayoutLoad } from "./$types";

export const load: LayoutLoad = async (event) => {
  await requireUser(event);
  const [teams, myTeam] = await Promise.all([required(api.listTeams()), optional(api.myTeam())]);
  return { teams, myTeam };
};
