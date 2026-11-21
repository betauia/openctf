import { redirect } from "@sveltejs/kit";
import * as api from "@library/api";
import { loginFor } from "@library/redirect";
import { optional, required, tolerated } from "@library/load";
import type { PageLoad } from "./$types";

export const load: PageLoad = async ({ parent, url }) => {
  const { user, signedOutBy } = await parent();
  if (!user) redirect(302, loginFor(url.pathname, signedOutBy ?? undefined));

  const [myTeam, teams] = await Promise.all([optional(api.myTeam()), required(api.listTeams())]);

  const memberId = url.searchParams.get("member");

  return {
    user,
    myTeam,
    teams,
    activity: myTeam ? await tolerated(api.teamActivity(myTeam.id), []) : { value: [], error: null },
    member: memberId ? await tolerated(api.memberDetail(memberId), null) : { value: null, error: null },
  };
};
