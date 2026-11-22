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

<aside
  class="shrink-0 bg-background border-b md:border-b-0 md:border-r border-text/7 flex md:flex-col md:w-40
    overflow-x-auto md:overflow-x-hidden md:overflow-y-auto px-2 md:px-0 py-2 md:py-3 [scrollbar-width:none]"
>
  {#each entries as { cat, label, total, pct, Icon } (cat)}
    <button
      onclick={() => (activeCat = cat)}
      class="group flex flex-col shrink-0 py-1.5 md:py-2.5 px-2 md:px-3 rounded-md md:rounded-none border-none
        bg-transparent text-text/45 cursor-pointer text-left md:w-full transition-colors duration-100 hover:bg-text/3"
      class:active={activeCat === cat}
    >
      <div class="flex items-center gap-2 md:gap-2.5 md:mb-2 whitespace-nowrap">
        <div
          class="w-7.5 h-7.5 bg-elevated rounded-md flex items-center justify-center shrink-0
            text-text/40 transition-colors duration-100
            group-[.active]:bg-[color-mix(in_srgb,var(--accent-color)_18%,var(--elevated-color))]
            group-[.active]:text-accent"
        >
          {#if Icon}<Icon size="0.8125rem" />{/if}
        </div>
        <span
          class="text-body tracking-[0.03em] text-text/55 transition-colors duration-100
            group-hover:text-text/80 group-[.active]:text-text/90"
        >
          {label} <em class="not-italic text-2xs text-text/20 ml-1">{total}</em>
        </span>
      </div>
      <div class="hidden md:block h-0.5 bg-elevated ml-10 overflow-hidden">
        <div class="h-full bg-accent transition-[width] duration-300 ease-out" style="width: {pct}%"></div>
      </div>
    </button>
  {/each}
</aside>
