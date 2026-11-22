<script lang="ts">
  import { CAT_ICON } from "@library/categories";
  import type { Challenge } from "@library/types";
  import { fmt, pad2 } from "@library/utils";
  import { ArrowDown, ArrowUp, ArrowUpDown } from "lucide-svelte";

  type Row = Challenge & { n: number };

  const COLLAPSE_KEY = "openctf_solved_collapsed";
  const DIFFICULTY = ["easy", "medium", "hard", "insane"];
  const COLUMNS = [
    { key: "n", label: "#" },
    { key: "title", label: "Challenges" },
    { key: "category", label: "Category" },
    { key: "solves", label: "Solves" },
    { key: "points", label: "Points" },
    { key: "difficulty", label: "Difficulty" },
  ] as const;

  type SortKey = (typeof COLUMNS)[number]["key"];

  let {
    rows,
    solved,
    onselect,
  }: {
    rows: Row[];
    solved: Set<string>;
    onselect: (challenge: Challenge) => void;
  } = $props();

  let collapsed = $state(load());
  let sort = $state<{ key: SortKey; desc: boolean }>({ key: "n", desc: false });

  const sortValue = (c: Row, key: SortKey) =>
    key === "difficulty" ? DIFFICULTY.indexOf(c.difficulty.toLowerCase()) : c[key];

  let sorted = $derived(
    rows.toSorted((a, b) => {
      const x = sortValue(a, sort.key);
      const y = sortValue(b, sort.key);
      const order = typeof x === "string" ? x.localeCompare(y as string) : x - (y as number);
      return sort.desc ? -order : order;
    }),
  );
  let unsolvedRows = $derived(sorted.filter((c) => !solved.has(c.id)));
  let solvedRows = $derived(sorted.filter((c) => solved.has(c.id)));

  function sortBy(key: SortKey) {
    sort = { key, desc: sort.key === key && !sort.desc };
  }

  function load() {
    try {
      return localStorage.getItem(COLLAPSE_KEY) === "true";
    } catch {
      return false;
    }
  }

  function toggle() {
    collapsed = !collapsed;
    try {
      localStorage.setItem(COLLAPSE_KEY, String(collapsed));
    } catch {}
  }
</script>

<div class="flex-1 min-h-0 overflow-auto p-2 sm:p-4">
  {#if rows.length === 0}
    <div class="empty">No challenges available</div>
  {:else}
    <table class="w-full min-w-[40rem] border-separate border-spacing-x-0 border-spacing-y-0.5">
      <thead>
        <tr class="border-b border-border-subtle">
          {#each COLUMNS as { key, label }, i (key)}
            {@const active = sort.key === key && !(key === "n" && !sort.desc)}
            {@const Icon = !active ? ArrowUpDown : sort.desc ? ArrowDown : ArrowUp}
            <th
              class="th {i === 1 ? 'text-left w-[40%]' : 'text-center'}"
              aria-sort={sort.key === key ? (sort.desc ? "descending" : "ascending") : "none"}
            >
              <button onclick={() => sortBy(key)} class="group relative inline-flex items-center cursor-pointer p-0">
                {label}
                <Icon
                  size="0.6875rem"
                  class="absolute left-full ml-1 transition-opacity duration-100
                    {active ? 'opacity-70' : 'opacity-0 group-hover:opacity-70'}"
                />
              </button>
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
            <td class="td text-label text-center w-9 {isSolved ? 'text-[var(--diff-easy)]' : 'text-text/18'}">
              {pad2(c.n)}
            </td>
            <td class="td text-body text-muted-text font-medium">{c.title}</td>
            <td class="td text-center">
              <span class="inline-flex items-center gap-1 py-0.5 px-1.75 text-2xs tracking-[0.07em] cat-{c.category}">
                {#if Icon}<Icon size="0.6875rem" />{/if}
                {fmt(c.category)}
              </span>
            </td>
            <td class="td text-xs text-text/45 text-center">{c.solves}</td>
            <td class="td text-body font-bold text-pts text-center">
              {c.points}
            </td>
            <td class="td text-center">
              <span
                class="inline-flex items-center gap-1 py-0.5 px-1.75 text-2xs tracking-[0.07em]
                  diff-{c.difficulty.toLowerCase()}"
              >
                {fmt(c.difficulty)}
              </span>
            </td>
          </tr>
        {/snippet}

        {#each unsolvedRows as c (c.id)}
          {@render row(c, false)}
        {/each}

        {#if solvedRows.length > 0}
          <tr>
            <td colspan="6" class="p-0 border-t border-b border-correct/18 bg-correct/6">
              <button
                onclick={toggle}
                style="color: var(--diff-easy)"
                class="flex items-center gap-1.5 bg-transparent border-none cursor-pointer
                  text-2xs tracking-[0.1em] py-1.5 px-3 w-full"
              >
                <span class="text-4xs inline-flex transition-transform duration-150" class:-rotate-90={collapsed}>
                  ▼
                </span>
                Solved
                <span class="text-correct/50">({solvedRows.length})</span>
              </button>
            </td>
          </tr>

          {#if !collapsed}
            {#each solvedRows as c (c.id)}
              {@render row(c, true)}
            {/each}
          {/if}
        {/if}
      </tbody>
    </table>
  {/if}
</div>
