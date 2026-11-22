import * as api from "@library/api";
import { optional, requireUser, tolerated } from "@library/load";
import type { PageLoad } from "./$types";

export const load: PageLoad = async (event) => {
  const { user, settings } = await requireUser(event);
  const { url } = event;

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
