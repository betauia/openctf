<script lang="ts">
  import Activity from "@components/teams/Activity.svelte";
  import type { ActivityEntry, Member, Team } from "@library/types";
  import { initial, pad2 } from "@library/utils";
  import { ArrowLeft } from "lucide-svelte";

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

<div class="flex-1 overflow-y-auto flex flex-col">
  {#if inspecting}
    <div class="flex items-center gap-3.5 py-4.5 px-6 bg-elevated border-b border-border-subtle shrink-0">
      <button onclick={onback} class="back-btn">
        <ArrowLeft size={14} />
        Back
      </button>
      <div class="avatar w-10 h-10 text-base">
        {initial(inspecting.name)}
      </div>
      <div>
        <div class="text-base font-bold text-text">{inspecting.name}</div>
        <div class="text-[11px] text-text/35 mt-0.75">
          {inspecting.score ?? "—"} pts · Rank #{inspecting.rank ?? "—"}
        </div>
      </div>
    </div>

    <div class="p-6 text-[13px] text-text/35">
      {inspecting.member_count} members. The roster is only returned for your own team.
    </div>

    {#if activityError || activity.length > 0}
      <Activity entries={activity} error={activityError} />
    {/if}
  {:else if teams.length === 0}
    <div class="empty">No teams yet</div>
  {:else}
    <div class="flex-1 overflow-y-auto p-4">
      <table class="w-full border-separate border-spacing-x-0 border-spacing-y-0.5">
        <thead>
          <tr>
            {#each ["#", "Team", "Members", "Score"] as header, i (header)}
              <th class="th {i >= 2 ? 'text-center' : 'text-left'}">
                {header}
              </th>
            {/each}
          </tr>
        </thead>
        <tbody>
          {#each teams as team, i (team.id)}
            {@const mine = team.id === myTeam?.id}
            <tr onclick={() => (mine ? onmine() : oninspect(team))} class="trow cursor-pointer">
              <td class="td text-[11px] w-9 {mine ? 'text-accent' : 'text-text/18'}">
                {pad2(i + 1)}
              </td>
              <td class="td text-[13px] font-medium {mine ? 'text-accent' : 'text-muted-text'}">
                {team.name}
              </td>
              <td class="td text-[12px] text-text/45 text-center">{team.member_count}</td>
              <td class="td text-[13px] font-bold text-pts text-center">{team.score ?? "—"}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>
