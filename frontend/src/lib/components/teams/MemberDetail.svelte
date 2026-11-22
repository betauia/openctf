<script lang="ts">
  import { CAT_ICON } from "@library/categories";
  import type { MemberDetail } from "@library/types";
  import { fmt, initial, timeAgo } from "@library/utils";
  import { ArrowLeft, ArrowRight, Users } from "lucide-svelte";

  let { member, backLabel, onback }: { member: MemberDetail; backLabel: string; onback: () => void } = $props();
</script>

<div class="w-full p-6 flex flex-col gap-4">
  <button onclick={onback} class="back-btn self-start">
    <ArrowLeft size="0.875rem" />
    {backLabel}
  </button>

  <section class="rounded-lg bg-elevated border border-text/6 p-6 flex items-center gap-5">
    <div class="avatar w-16 h-16 text-2xl rounded-lg">{initial(member.username)}</div>
    <div class="flex-1 min-w-0">
      <h1 class="m-0 text-2xl font-bold text-text truncate">{member.username}</h1>
      <div class="flex flex-wrap items-center gap-2 mt-2.5">
        <span
          class="inline-flex items-center text-[0.6875rem] font-bold text-pts bg-pts/8 border border-pts/20 rounded-full
            py-1 px-2.5 tabular-nums"
        >
          {member.score} pts
        </span>
        <span
          class="inline-flex items-center text-[0.6875rem] text-text/60 bg-text/5 border border-text/8 rounded-full
            py-1 px-2.5"
        >
          {member.solves.length}
          {member.solves.length === 1 ? "solve" : "solves"}
        </span>
        {#if member.team && member.team_id}
          <a
            href="/teams/scout?team={member.team_id}"
            title="Inspect {member.team}"
            class="group inline-flex items-center gap-1.5 text-[0.6875rem] text-text/70 no-underline bg-text/5
              border border-text/8 rounded-full py-1 px-2.5 transition-colors duration-100
              hover:text-text hover:border-text/20"
          >
            <Users size="0.6875rem" />
            {member.team}
            <ArrowRight size="0.6875rem" class="text-text/30 transition-colors duration-100 group-hover:text-accent" />
          </a>
        {:else}
          <span
            class="inline-flex items-center gap-1.5 text-[0.6875rem] text-text/35 border border-dashed border-text/10
              rounded-full py-1 px-2.5"
          >
            <Users size="0.6875rem" />
            No team
          </span>
        {/if}
      </div>
    </div>
  </section>

  <section class="rounded-lg bg-elevated border border-text/6 overflow-hidden">
    <div class="px-5 pt-4 pb-3 text-[0.8125rem] font-medium text-text/80">Solves</div>
    {#if member.solves.length === 0}
      <div class="text-center py-10 text-[0.75rem] text-text/25 border-t border-text/5">No solves yet</div>
    {:else}
      {#each member.solves as solve (solve.challenge_id)}
        {@const Icon = CAT_ICON[solve.category]}
        <a
          href="/challenges?open={solve.challenge_id}"
          title="Open {solve.challenge}"
          class="group flex items-center gap-4 px-5 py-3 border-t border-text/5 no-underline
            transition-colors duration-100 hover:bg-text/4"
        >
          <span
            class="flex items-center justify-center w-8 h-8 rounded-md shrink-0 cat-{solve.category}"
            title={fmt(solve.category)}
          >
            {#if Icon}<Icon size="0.875rem" />{/if}
          </span>
          <div class="flex-1 min-w-0">
            <div class="text-[0.8125rem] font-medium text-muted-text truncate group-hover:text-text">
              {solve.challenge}
            </div>
            <div class="text-[0.6875rem] text-text/30 mt-0.5">{fmt(solve.category)}</div>
          </div>
          <span class="text-[0.8125rem] font-bold text-pts tabular-nums">+{solve.points}</span>
          <span class="w-16 text-right text-[0.6875rem] text-text/30 shrink-0">{timeAgo(solve.solved_at)}</span>
          <ArrowRight
            size="0.8125rem"
            class="text-text/15 shrink-0 transition-colors duration-100 group-hover:text-accent"
          />
        </a>
      {/each}
    {/if}
  </section>
</div>
