<script lang="ts">
  import Activity from "@components/teams/Activity.svelte";
  import type { ActivityEntry, Member, Team } from "@library/types";
  import { initial } from "@library/utils";
  import { ArrowLeft, Trophy, Users } from "lucide-svelte";

  let {
    teams,
    myTeam,
    inspecting,
    activity,
    activityError,
    onmine,
    oninspect,
    onback,
    onmember,
  }: {
    teams: Team[];
    myTeam: Team | null;
    /** Set when a team row is opened; null shows the leaderboard list. */
    inspecting: Team | null;
    activity: ActivityEntry[];
    activityError: string | null;
    onmine: () => void;
    oninspect: (team: Team) => void;
    onback: () => void;
    onmember: (member: Member) => void;
  } = $props();
</script>

<div class="w-full p-6 flex flex-col gap-4">
  {#if inspecting}
    <button onclick={onback} class="back-btn self-start">
      <ArrowLeft size="0.875rem" />
      Back
    </button>

    <section class="rounded-lg bg-elevated border border-text/6 p-6 flex items-center gap-5">
      <div class="avatar w-16 h-16 text-2xl rounded-lg">{initial(inspecting.name)}</div>
      <div class="flex-1 min-w-0">
        <h1 class="m-0 text-2xl font-bold text-text truncate">{inspecting.name}</h1>
        <div class="flex flex-wrap items-center gap-2 mt-2.5">
          <span
            class="inline-flex items-center gap-1.5 text-[0.6875rem] text-text/70 bg-text/5 border border-text/8
              rounded-full py-1 px-2.5"
          >
            <Trophy size="0.6875rem" class="text-pts" />
            Rank #{inspecting.rank}
          </span>
          <span
            class="inline-flex items-center text-[0.6875rem] font-bold text-pts bg-pts/8 border border-pts/20
              rounded-full py-1 px-2.5 tabular-nums"
          >
            {inspecting.score} pts
          </span>
          <span
            class="inline-flex items-center gap-1.5 text-[0.6875rem] text-text/60 bg-text/5 border border-text/8
              rounded-full py-1 px-2.5"
          >
            <Users size="0.6875rem" />
            {inspecting.member_count}
            {inspecting.member_count === 1 ? "member" : "members"}
          </span>
        </div>
      </div>
    </section>

    <Activity entries={activity} error={activityError} />
  {:else if teams.length === 0}
    <div class="rounded-lg bg-elevated border border-text/6 text-center py-12 text-[0.75rem] text-text/25">
      No teams yet
    </div>
  {:else}
    <section class="rounded-lg bg-elevated border border-text/6 overflow-hidden">
      <div class="flex items-center justify-between px-5 pt-4 pb-3">
        <div class="text-[0.8125rem] font-medium text-text/80">Leaderboard</div>
        <div class="text-[0.6875rem] text-text/30">{teams.length} {teams.length === 1 ? "team" : "teams"}</div>
      </div>
      {#each teams as team, i (team.id)}
        {@const mine = team.id === myTeam?.id}
        <div
          role="button"
          tabindex="0"
          onclick={() => (mine ? onmine() : oninspect(team))}
          onkeydown={(e) => e.key === "Enter" && (mine ? onmine() : oninspect(team))}
          class="flex items-center gap-4 px-5 py-3 border-t border-text/5 cursor-pointer transition-colors
            duration-100 hover:bg-text/3 {mine ? 'bg-accent/5' : ''}"
        >
          <span
            class="w-7 text-center text-[0.75rem] font-bold tabular-nums
              {i === 0 ? 'text-pts' : i < 3 ? 'text-text/60' : 'text-text/25'}"
          >
            {i + 1}
          </span>
          <div class="avatar w-9 h-9 text-[0.8125rem] rounded-md">{initial(team.name)}</div>
          <div class="flex-1 min-w-0 flex items-center gap-2">
            <span class="text-[0.8125rem] font-medium truncate {mine ? 'text-accent' : 'text-muted-text'}">
              {team.name}
            </span>
            {#if mine}
              <span
                class="text-[0.5625rem] tracking-[0.1em] text-accent bg-accent/12 border border-accent/28 py-px px-1.5"
              >
                your team
              </span>
            {/if}
          </div>
          <span class="hidden sm:inline-flex items-center gap-1.5 text-[0.6875rem] text-text/35 w-24">
            <Users size="0.6875rem" />
            {team.member_count}
            {team.member_count === 1 ? "member" : "members"}
          </span>
          <span class="w-16 text-right text-[0.8125rem] font-bold text-pts tabular-nums">{team.score}</span>
        </div>
      {/each}
    </section>
  {/if}
</div>
