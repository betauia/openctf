<script lang="ts">
  import type { ActivityEntry } from "@library/types";
  import { initial, timeAgo } from "@library/utils";

  let { entries, error = null }: { entries: ActivityEntry[]; error?: string | null } = $props();
</script>

<div class="border-t border-border-subtle flex flex-col">
  <div class="text-[9px] tracking-[0.14em] text-text/25 pt-2.5 px-6 pb-1.5 shrink-0">Recent Activity</div>
  <div class="max-h-60 overflow-y-auto">
    {#if error}
      <div class="empty text-wrong/70">{error}</div>
    {:else if entries.length === 0}
      <div class="empty">No activity yet</div>
    {:else}
      {#each entries as entry (entry.username + entry.challenge)}
        <div class="flex items-start gap-3 py-2 px-6 border-b border-border-subtle last:border-b-0">
          <div class="avatar w-6 h-6 text-[10px] mt-px">
            {initial(entry.username)}
          </div>
          <div class="flex flex-col gap-0.75">
            <div class="flex items-center flex-wrap gap-1.5 text-[12px]">
              <span class="text-muted-text font-semibold">{entry.username}</span>
              <span class="text-text/30">solved</span>
              <span class="text-[11px] py-px px-1.75 cat-{entry.category}">{entry.challenge}</span>
              <span class="text-[11px] font-bold text-pts">+{entry.points}</span>
            </div>
            <div class="text-[10px] text-text/20 tracking-[0.04em]">{timeAgo(entry.solved_at)}</div>
          </div>
        </div>
      {/each}
    {/if}
  </div>
</div>
