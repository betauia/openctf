import { redirect } from "@sveltejs/kit";
import * as api from "@library/api";
import { loginFor } from "@library/redirect";
import { optional, tolerated } from "@library/load";
import type { PageLoad } from "./$types";

export const load: PageLoad = async ({ parent, url }) => {
  const { user, signedOutBy, settings } = await parent();
  if (!user) redirect(302, loginFor(url.pathname, signedOutBy ?? undefined));

  const memberId = url.searchParams.get("member");
  const myTeam = await optional(api.myTeam());

  const [progress, activity, member] = await Promise.all([
    myTeam ? tolerated(api.teamProgress(myTeam.id), null) : { value: null, error: null },
    myTeam ? tolerated(api.teamActivity(myTeam.id), []) : { value: [], error: null },
    memberId ? tolerated(api.memberDetail(memberId), null) : { value: null, error: null },
  ]);

  return {
    user,
    myTeam,
    maxTeamSize: settings?.max_team_size ?? null,
    progress: progress.value,
    activity,
    member,
  };
};
