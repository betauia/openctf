<script lang="ts">
  import { CAT_ICON } from "@library/categories";
  import type { Challenge } from "@library/types";
  import { fmt, pad2 } from "@library/utils";

  type Row = Challenge & { n: number };

  let {
    unsolved,
    solved,
    collapsed = $bindable(),
    onselect,
  }: {
    unsolved: Row[];
    solved: Row[];
    collapsed: boolean;
    onselect: (challenge: Challenge) => void;
  } = $props();
</script>

<div class="flex-1 overflow-y-auto p-4">
  {#if unsolved.length === 0 && solved.length === 0}
    <div class="empty">No challenges available</div>
  {:else}
    <table class="w-full border-separate border-spacing-x-0 border-spacing-y-0.5">
      <thead>
        <tr class="border-b border-border-subtle">
          {#each ["#", "Challenges", "Category", "Solves", "Points", "Difficulty"] as header, i (header)}
            <th class="th {i >= 2 ? 'text-center' : 'text-left'}">
              {header}
            </th>
          {/each}
        </tr>
      </thead>

      <tbody>
        {#snippet row(c: Row, isSolved: boolean)}
          {@const Icon = CAT_ICON[c.category]}
          <tr
            onclick={() => onselect(c)}
            class="trow transition-colors duration-100 cursor-pointer"
            class:opacity-50={isSolved}
          >
            <td class="td text-[11px] w-9 {isSolved ? 'text-[var(--diff-easy)]' : 'text-text/18'}">
              {pad2(c.n)}
            </td>
            <td class="td text-[13px] text-muted-text font-medium">{c.title}</td>
            <td class="td text-center">
              <span
                class="inline-flex items-center gap-1 py-0.5 px-1.75 text-[10px] tracking-[0.07em] cat-{c.category}"
              >
                {#if Icon}<Icon size={11} />{/if}
                {fmt(c.category)}
              </span>
            </td>
            <td class="td text-[12px] text-text/45 text-center">{c.solves}</td>
            <td class="td text-[13px] font-bold text-pts text-center">{c.points}</td>
            <td class="td text-center">
              <span
                class="inline-flex items-center gap-1 py-0.5 px-1.75 text-[10px] tracking-[0.07em]
                  diff-{c.difficulty.toLowerCase()}"
              >
                {fmt(c.difficulty)}
              </span>
            </td>
          </tr>
        {/snippet}

        {#each unsolved as c (c.id)}
          {@render row(c, false)}
        {/each}

        {#if solved.length > 0}
          <tr>
            <td colspan="6" class="p-0 border-t border-b border-correct/18 bg-correct/6">
              <button
                onclick={() => (collapsed = !collapsed)}
                style="color: var(--diff-easy)"
                class="flex items-center gap-1.5 bg-transparent border-none cursor-pointer
                  text-[10px] tracking-[0.1em] py-1.5 px-3 w-full"
              >
                <span class="text-[8px] inline-flex transition-transform duration-150" class:-rotate-90={collapsed}>
                  ▼
                </span>
                Solved
                <span class="text-correct/50">({solved.length})</span>
              </button>
            </td>
          </tr>

          {#if !collapsed}
            {#each solved as c (c.id)}
              {@render row(c, true)}
            {/each}
          {/if}
        {/if}
      </tbody>
    </table>
  {/if}
</div>
