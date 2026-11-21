<script lang="ts">
  import { CAT_ICON } from "@library/categories";
  import type { Challenge } from "@library/types";
  import { fmt } from "@library/utils";
  import { Hash } from "lucide-svelte";

  let {
    challenges,
    solved,
    activeCat = $bindable(),
  }: { challenges: Challenge[]; solved: Set<string>; activeCat: string } = $props();

  let entries = $derived(
    ["all", ...new Set(challenges.map((c) => c.category))].map((cat) => {
      const inCat = cat === "all" ? challenges : challenges.filter((c) => c.category === cat);
      const done = inCat.filter((c) => solved.has(c.id)).length;
      return {
        cat,
        label: cat === "all" ? "All" : fmt(cat),
        total: inCat.length,
        pct: inCat.length > 0 ? (done / inCat.length) * 100 : 0,
        Icon: cat === "all" ? Hash : CAT_ICON[cat],
      };
    }),
  );
</script>

<aside class="w-40 shrink-0 bg-background border-r border-text/7 flex flex-col overflow-y-auto py-3">
  <div class="px-3.5 pb-3 text-base font-bold text-text text-center border-b border-border-subtle mb-1.5">
    {challenges.filter((c) => solved.has(c.id)).length}
    <span class="text-[12px] font-normal text-muted-text opacity-40">/ {challenges.length} solved</span>
  </div>

  {#each entries as { cat, label, total, pct, Icon } (cat)}
    <button
      onclick={() => (activeCat = cat)}
      class="group flex flex-col py-2.5 px-3 border-none bg-transparent text-text/45 cursor-pointer
        text-left w-full transition-colors duration-100 hover:bg-text/3"
      class:active={activeCat === cat}
    >
      <div class="flex items-center gap-2.5 mb-2">
        <div
          class="w-7.5 h-7.5 bg-elevated rounded-md flex items-center justify-center shrink-0
            text-text/40 transition-colors duration-100
            group-[.active]:bg-[color-mix(in_srgb,var(--accent-color)_18%,var(--elevated-color))]
            group-[.active]:text-accent"
        >
          {#if Icon}<Icon size={13} />{/if}
        </div>
        <span
          class="text-[13px] tracking-[0.03em] text-text/55 transition-colors duration-100
            group-hover:text-text/80 group-[.active]:text-text/90"
        >
          {label} <em class="not-italic text-[10px] text-text/20 ml-1">{total}</em>
        </span>
      </div>
      <div class="h-0.5 bg-elevated ml-10 overflow-hidden">
        <div class="h-full bg-accent transition-[width] duration-300 ease-out" style="width: {pct}%"></div>
      </div>
    </button>
  {/each}
</aside>
