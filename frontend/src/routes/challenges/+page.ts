import { redirect } from "@sveltejs/kit";
import * as api from "@library/api";
import { loginFor } from "@library/redirect";
import { optional, required, tolerated } from "@library/load";
import type { PageLoad } from "./$types";

export const load: PageLoad = async ({ parent, url }) => {
  const { user, signedOutBy, settings } = await parent();
  if (!user) redirect(302, loginFor(url.pathname, signedOutBy ?? undefined));

  const [challenges, mySolves, myTeam, teams, players] = await Promise.all([
    required(api.listChallenges()),
    required(api.mySolves()),
    optional(api.myTeam()),
    tolerated(api.listTeams(), []),
    tolerated(api.listPlayers(), []),
  ]);

  const teamSolves = myTeam ? await tolerated(api.teamActivity(myTeam.id), null) : null;
  const solvedIds = teamSolves?.value ? [...new Set(teamSolves.value.map((a) => a.challenge_id))] : mySolves;

  return { user, challenges, solvedIds, myTeam, settings, teams: teams.value, players: players.value };
};
