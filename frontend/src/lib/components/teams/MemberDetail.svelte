<script lang="ts">
  import { CAT_ICON } from "#library/categories.js";
  import type { MemberDetail } from "#library/types.js";
  import { fmt, initial, timeAgo } from "#library/utils.js";
  import { ArrowLeft, ArrowRight, Users } from "lucide-svelte";

  let { member }: { member: MemberDetail } = $props();
</script>

<div class="w-full p-6 flex flex-col gap-4">
  <button onclick={() => history.back()} class="back-btn self-start">
    <ArrowLeft size="0.875rem" />
    Back
  </button>

  <section class="card p-6 flex items-center gap-5">
    <div class="avatar w-16 h-16 text-2xl rounded-lg">
      {initial(member.username)}
    </div>
    <div class="flex-1 min-w-0">
      <h1 class="m-0 text-2xl font-bold text-text truncate">
        {member.username}
      </h1>
      <div class="flex flex-wrap items-center gap-2 mt-2.5">
        <span class="pill font-bold text-pts bg-pts/8 border-pts/20 tabular-nums">
          {member.score} pts
        </span>
        <span class="pill text-text/60">
          {member.solves.length}
          {member.solves.length === 1 ? "solve" : "solves"}
        </span>
        {#if member.team && member.team_id}
          <a
            href="/teams/scout?team={member.team_id}"
            title="Inspect {member.team}"
            class="group pill text-text/70 no-underline transition-colors duration-100 hover:text-text hover:border-text/20"
          >
            <Users size="0.6875rem" />
            {member.team}
            <ArrowRight size="0.6875rem" class="text-text/30 transition-colors duration-100 group-hover:text-accent" />
          </a>
        {:else}
          <span class="pill text-text/35 bg-transparent border-dashed border-text/10">
            <Users size="0.6875rem" />
            No team
          </span>
        {/if}
      </div>
    </div>
  </section>

  <section class="card overflow-hidden">
    <div class="px-5 pt-4 pb-3 card-title">Solves</div>
    {#if member.solves.length === 0}
      <div class="text-center py-10 text-xs text-text/25 border-t border-text/5">No solves yet</div>
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
            <div class="text-body font-medium text-muted-text truncate group-hover:text-text">
              {solve.challenge}
            </div>
            <div class="text-label text-text/30 mt-0.5">
              {fmt(solve.category)}
            </div>
          </div>
          <span class="text-body font-bold text-pts tabular-nums">
            +{solve.points}
          </span>
          <span class="w-16 text-right text-label text-text/30 shrink-0">
            {timeAgo(solve.solved_at)}
          </span>
          <ArrowRight
            size="0.8125rem"
            class="text-text/15 shrink-0 transition-colors duration-100 group-hover:text-accent"
          />
        </a>
      {/each}
    {/if}
  </section>
</div>
