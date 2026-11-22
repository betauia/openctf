import * as api from "@library/api";
import { optional, requireUser, required, tolerated } from "@library/load";
import type { PageLoad } from "./$types";

export const load: PageLoad = async (event) => {
  const { user, settings } = await requireUser(event);

  const [challenges, mySolves, myTeam, teams, players] = await Promise.all([
    required(api.listChallenges()),
    required(api.mySolves()),
    optional(api.myTeam()),
    tolerated(api.listTeams(), []),
    tolerated(api.listPlayers(), []),
  ]);

  const teamSolves = myTeam ? await tolerated(api.teamActivity(myTeam.id), null) : null;
  const solvedIds = teamSolves?.value?.map((a) => a.challenge_id) ?? mySolves;

  return { user, challenges, solvedIds, myTeam, settings, teams: teams.value, players: players.value };
};
