<script lang="ts">
  import type { ActivityEntry } from "#library/types.js";
  import { initial, timeAgo } from "#library/utils.js";
  import { ArrowRight } from "lucide-svelte";

  let { entries, error = null }: { entries: ActivityEntry[]; error?: string | null } = $props();
</script>

<div class="card flex flex-col">
  <div class="flex items-center justify-between px-5 pt-4 pb-3">
    <div class="card-title">Recent Activity</div>
    {#if entries.length > 0}
      <div class="text-label text-text/30">{entries.length} solves</div>
    {/if}
  </div>
  <div class="max-h-80 overflow-y-auto">
    {#if error}
      <div class="text-center py-10 text-xs text-wrong/70">{error}</div>
    {:else if entries.length === 0}
      <div class="text-center py-10 text-xs text-text/25">No solves yet</div>
    {:else}
      {#each entries as entry (entry.username + entry.challenge_id)}
        <a
          href="/challenges?open={entry.challenge_id}"
          title="Open {entry.challenge}"
          class="group flex items-center gap-3 py-2.5 px-5 border-t border-text/5 no-underline
            transition-colors duration-100 hover:bg-text/4"
        >
          <div class="avatar w-7 h-7 text-label rounded">
            {initial(entry.username)}
          </div>
          <div class="flex items-center flex-wrap gap-1.5 text-xs flex-1 min-w-0">
            <span class="text-muted-text font-semibold">{entry.username}</span>
            <span class="text-text/30">solved</span>
            <span class="text-label py-px px-1.75 cat-{entry.category} group-hover:underline underline-offset-2">
              {entry.challenge}
            </span>
          </div>
          <span class="text-xs font-bold text-pts tabular-nums">
            +{entry.points}
          </span>
          <span class="text-label text-text/25 w-16 text-right shrink-0">
            {timeAgo(entry.solved_at)}
          </span>
          <ArrowRight
            size="0.75rem"
            class="text-text/15 shrink-0 transition-colors duration-100 group-hover:text-accent"
          />
        </a>
      {/each}
    {/if}
  </div>
</div>
