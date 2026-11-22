import * as api from "@library/api";
import { optional, requireUser, required } from "@library/load";
import type { PageLoad } from "./$types";

const COLORS = [
  "--accent-color",
  "--accent-secondary",
  "--cat-misc",
  "--cat-rev",
  "--cat-crypto",
  "--cat-beginner",
  "--cat-osint",
  "--cat-hardware",
  "--cat-forensics",
  "--cat-stego",
];

export const load: PageLoad = async (event) => {
  const { user, settings } = await requireUser(event);

  const [teams, myTeam, scoreboard] = await Promise.all([
    required(api.listTeams()),
    optional(api.myTeam()),
    required(api.scoreboard()),
  ]);

  const series = scoreboard.map((s, i) => ({
    id: s.team_id,
    name: s.team_name,
    color: `var(${COLORS[i % COLORS.length]})`,
    points: s.points.map((p) => [Date.parse(p.at), p.score] as [number, number]),
  }));

  const start = settings?.ctf_start ? Date.parse(settings.ctf_start) : null;
  const end = Math.min(Date.now(), settings?.ctf_end ? Date.parse(settings.ctf_end) : Date.now());

  return { user, teams, myTeam, series, start, end };
};
