import * as api from "#library/api.js";
import { optional, requireUser, required, tolerated } from "#library/load.js";
import type { PageLoad } from "./$types";

export const load: PageLoad = async (event) => {
  const { user, settings } = await requireUser(event);

  const [challenges, solvedIds, myTeam, teams, players] = await Promise.all([
    required(api.listChallenges()),
    required(api.mySolves()),
    optional(api.myTeam()),
    tolerated(api.listTeams(), []),
    tolerated(api.listPlayers(), []),
  ]);

  return { user, challenges, solvedIds, myTeam, settings, teams: teams.value, players: players.value };
};
