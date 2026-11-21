import { redirect } from "@sveltejs/kit";
import * as api from "@library/api";
import { loginFor } from "@library/redirect";
import { optional, required } from "@library/load";
import type { PageLoad } from "./$types";

export const load: PageLoad = async ({ parent, url }) => {
  const { user, signedOutBy } = await parent();
  if (!user) redirect(302, loginFor(url.pathname, signedOutBy ?? undefined));

  const [challenges, solvedIds, myTeam] = await Promise.all([
    required(api.listChallenges()),
    required(api.mySolves()),
    optional(api.myTeam()),
  ]);

  return { user, challenges, solvedIds, myTeam };
};
