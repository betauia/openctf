import * as api from "#library/api.js";
import type { PageLoad } from "./$types";

export const load: PageLoad = async ({ parent, url }) => {
  const { teams, myTeam } = await parent();
  const id = url.searchParams.get("team") ?? teams.find((t) => t.id !== myTeam?.id)?.id;
  const selected = teams.find((t) => t.id === id) ?? null;

  const scouted = selected
    ? Promise.all([api.teamActivity(selected.id), api.compareTeam(selected.id)]).then(
        ([entries, comparison]) => ({ entries, comparison, error: null }),
        (e) => ({ entries: [], comparison: null, error: e instanceof Error ? e.message : "Could not load this team" }),
      )
    : null;

  return { selected, scouted };
};
