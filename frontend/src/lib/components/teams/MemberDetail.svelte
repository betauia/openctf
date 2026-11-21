<script lang="ts">
  import type { MemberDetail } from "@library/types";
  import { initial, pad2, timeAgo } from "@library/utils";
  import { ArrowLeft } from "lucide-svelte";

  let { member, backLabel, onback }: { member: MemberDetail; backLabel: string; onback: () => void } = $props();
</script>

<div class="flex items-center gap-3.5 py-4.5 px-6 bg-elevated border-b border-border-subtle shrink-0">
  <button onclick={onback} class="back-btn">
    <ArrowLeft size={14} />
    {backLabel}
  </button>
  <div class="avatar w-10 h-10 text-base">
    {initial(member.username)}
  </div>
  <div>
    <div class="text-base font-bold text-text">{member.username}</div>
    <div class="text-[11px] text-text/35 mt-0.75">
      {member.score} pts · {member.solves.length} solved
    </div>
  </div>
</div>

{#if member.solves.length === 0}
  <div class="empty">No solves yet</div>
{:else}
  <div class="flex-1 overflow-y-auto p-4">
    <table class="w-full border-separate border-spacing-x-0 border-spacing-y-0.5">
      <thead>
        <tr>
          {#each ["#", "Challenge", "Category", "Points", "Solved"] as header, i (header)}
            <th class="th {i >= 2 ? 'text-center' : 'text-left'}">
              {header}
            </th>
          {/each}
        </tr>
      </thead>
      <tbody>
        {#each member.solves as solve, i (solve.challenge)}
          <tr>
            <td class="td text-[11px] text-text/18 w-9">{pad2(i + 1)}</td>
            <td class="td text-[13px] text-muted-text font-medium">{solve.challenge}</td>
            <td class="td text-center">
              <span
                class="inline-flex items-center gap-1 py-0.5 px-1.75 text-[10px] tracking-[0.07em] cat-{solve.category}"
              >
                {solve.category}
              </span>
            </td>
            <td class="td text-[13px] font-bold text-pts text-center">{solve.points}</td>
            <td class="td text-[12px] text-text/45 text-center">{timeAgo(solve.solved_at)}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}
