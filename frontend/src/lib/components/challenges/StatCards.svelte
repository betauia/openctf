<script lang="ts">
  import type { MyTeam, Player, PublicSettings, Team, User as UserType } from "@library/types";
  import { pad2 } from "@library/utils";
  import { ArrowDown, ArrowRight, ArrowUp, Clock, Target, User, Users } from "lucide-svelte";

  let {
    user,
    myTeam,
    teams,
    players,
    settings,
    solvedCount,
    total,
  }: {
    user: UserType;
    myTeam: MyTeam | null;
    teams: Team[];
    players: Player[];
    settings: PublicSettings | null;
    solvedCount: number;
    total: number;
  } = $props();

  let progressPct = $derived(total ? Math.round((solvedCount / total) * 100) : 0);

  let now = $state(Date.now());
  $effect(() => {
    const timer = setInterval(() => (now = Date.now()), 1000);
    return () => clearInterval(timer);
  });

  function countdown(ms: number) {
    const seconds = Math.floor(ms / 1000);
    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    const clock = [hours, minutes, seconds % 60].map(pad2).join(":");
    return days > 0 ? `${days}d ${clock}` : clock;
  }

  const when = (t: number) =>
    new Date(t).toLocaleString([], { day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" });

  let clock = $derived.by(() => {
    const start = settings?.ctf_start ? Date.parse(settings.ctf_start) : null;
    const end = settings?.ctf_end ? Date.parse(settings.ctf_end) : null;
    if (start !== null && now < start)
      return { label: "Starts in", value: countdown(start - now), sub: `Starts ${when(start)}` };
    if (end === null) return { label: "Time left", value: "—", sub: "" };
    if (now < end) return { label: "Time left", value: countdown(end - now), sub: `Ends ${when(end)}` };
    return { label: "Time left", value: "Ended", sub: `Ended ${when(end)}` };
  });

  function pointsGap(list: { id: string; score: number }[], id: string | undefined): number | null {
    const index = list.findIndex((entry) => entry.id === id);
    const neighbour = index === -1 ? undefined : list[index === 0 ? 1 : index - 1];
    return neighbour ? list[index].score - neighbour.score : null;
  }

  let userGap = $derived(pointsGap(players, user.id));
  let teamGap = $derived(pointsGap(teams, myTeam?.id));
</script>

{#snippet gapPill(gap: number | null)}
  {#if gap !== null}
    <span
      class="pill gap-1 shrink-0 py-px px-2 font-medium tabular-nums
        {gap > 0
        ? 'text-correct bg-correct/10 border-correct/25'
        : gap < 0
          ? 'text-wrong bg-wrong/10 border-wrong/25'
          : 'text-muted-text/70 bg-text/5 border-text/10'}"
    >
      {#if gap > 0}
        <ArrowUp size="0.6875rem" class="shrink-0" />
        {gap} points ahead
      {:else if gap < 0}
        <ArrowDown size="0.6875rem" class="shrink-0" />
        {-gap} points behind
      {:else}
        Tied
      {/if}
    </span>
  {/if}
{/snippet}

<div class="grid grid-cols-2 lg:grid-cols-4 gap-3 px-4 pt-4 shrink-0">
  <div class="min-w-0 card px-4 py-3">
    <div class="flex items-center gap-1.5 text-label text-text/45">
      <Clock size="0.75rem" />
      {clock.label}
    </div>
    <div class="mt-1 text-xl font-bold text-text tabular-nums truncate">
      {clock.value}
    </div>
    <div class="mt-0.5 text-label text-muted-text/60 truncate">{clock.sub}</div>
  </div>

  <div class="min-w-0 card px-4 py-3">
    <div class="flex items-center gap-1.5 text-label text-text/45">
      <Target size="0.75rem" />
      Progress
    </div>
    <div class="mt-1 flex items-baseline gap-1.5">
      <span class="text-xl font-bold text-text tabular-nums">
        {solvedCount}
      </span>
      <span class="text-xs text-text/35">/ {total} solved</span>
      <span class="ml-auto text-label text-text/35 tabular-nums">
        {progressPct}%
      </span>
    </div>
    <div class="mt-2 h-1 rounded-full bg-background overflow-hidden">
      <div class="h-full rounded-full bg-accent transition-[width] duration-300" style="width: {progressPct}%"></div>
    </div>
  </div>

  <div class="min-w-0 card px-4 py-3">
    <div class="flex items-center gap-1.5 text-label text-text/45">
      <User size="0.75rem" />
      You
      <span class="ml-auto flex items-center">
        {@render gapPill(userGap)}
      </span>
    </div>
    <div class="mt-1 text-xl font-bold text-text tabular-nums truncate">
      Rank #{user.rank}
    </div>
    <div class="mt-0.5 text-label text-muted-text/60 tabular-nums truncate">
      of all players · <span class="text-pts">{user.score} pts</span>
    </div>
  </div>

  {#if myTeam}
    <a
      href="/teams"
      class="group min-w-0 card px-4 py-3 no-underline
        transition-colors duration-100 hover:border-text/15"
    >
      <div class="flex items-center gap-1.5 text-label text-text/45">
        <Users size="0.75rem" />
        Team
        <span class="ml-auto flex items-center">
          {#if teamGap !== null}
            {@render gapPill(teamGap)}
          {:else}
            <ArrowRight size="0.75rem" class="text-text/20 transition-colors duration-100 group-hover:text-accent" />
          {/if}
        </span>
      </div>
      <div class="mt-1 text-xl font-bold text-text tabular-nums truncate">
        Rank #{myTeam.rank}
      </div>
      <div class="mt-0.5 text-label text-muted-text/60 tabular-nums truncate">
        {myTeam.name} · <span class="text-pts">{myTeam.score} pts</span>
      </div>
    </a>
  {:else}
    <a
      href="/teams"
      class="group min-w-0 rounded-lg border border-dashed border-text/12 px-4 py-3 no-underline
        transition-colors duration-100 hover:border-text/25"
    >
      <div class="flex items-center gap-1.5 text-label text-text/45">
        <Users size="0.75rem" />
        Team
      </div>
      <div class="mt-1 text-sm font-medium text-text/70 group-hover:text-text">No team yet</div>
      <div class="mt-0.5 inline-flex items-center gap-1 text-label text-accent">
        Create or join a team
        <ArrowRight size="0.6875rem" />
      </div>
    </a>
  {/if}
</div>
