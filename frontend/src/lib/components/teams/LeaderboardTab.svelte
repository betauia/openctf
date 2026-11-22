<script lang="ts">
  import type { Team } from "#library/types.js";
  import { initial } from "#library/utils.js";
  import { Users } from "lucide-svelte";

  let { teams, myTeam }: { teams: Team[]; myTeam: Team | null } = $props();
</script>

<div class="w-full p-6 flex flex-col gap-4">
  <section class="card overflow-hidden">
    <div class="flex items-center justify-between px-5 pt-4 pb-3">
      <div class="card-title">Leaderboard</div>
      <div class="text-label text-text/30">
        {teams.length}
        {teams.length === 1 ? "team" : "teams"}
      </div>
    </div>
    {#each teams as team (team.id)}
      {@const mine = team.id === myTeam?.id}
      <a
        href={mine ? "/teams" : `/teams/scout?team=${team.id}`}
        class="flex items-center gap-4 px-5 py-3 border-t border-text/5 no-underline transition-colors
          duration-100 hover:bg-text/3 {mine ? 'bg-accent/5' : ''}"
      >
        <span
          class="w-7 text-center text-xs font-bold tabular-nums
            {team.rank === 1 ? 'text-pts' : team.rank <= 3 ? 'text-text/60' : 'text-text/25'}"
        >
          {team.rank}
        </span>
        <div class="avatar w-9 h-9 text-body rounded-md">
          {initial(team.name)}
        </div>
        <div class="flex-1 min-w-0 flex items-center gap-2">
          <span class="text-body font-medium truncate {mine ? 'text-accent' : 'text-muted-text'}">
            {team.name}
          </span>
          {#if mine}
            <span class="text-3xs tracking-[0.1em] text-accent bg-accent/12 border border-accent/28 py-px px-1.5">
              your team
            </span>
          {/if}
        </div>
        <span class="hidden sm:inline-flex items-center gap-1.5 text-label text-text/35 w-24">
          <Users size="0.6875rem" />
          {team.member_count}
          {team.member_count === 1 ? "member" : "members"}
        </span>
        <span class="w-16 text-right text-body font-bold text-pts tabular-nums">
          {team.score}
        </span>
      </a>
    {/each}
  </section>
</div>
