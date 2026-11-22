<script lang="ts">
  import { goto } from "$app/navigation";
  import { navigating } from "$app/state";
  import Activity from "@components/teams/Activity.svelte";
  import { CAT_ICON } from "@library/categories";
  import { fmt, initial } from "@library/utils";
  import { ArrowLeft, ArrowRight, Search, Trophy, Users } from "lucide-svelte";
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  let search = $state("");

  let myScore = $derived(data.myTeam?.score ?? 0);
  let shown = $derived(data.teams.filter((t) => t.name.toLowerCase().includes(search.trim().toLowerCase())));
  let selected = $derived(data.selected);
  let highlighted = $derived(navigating.to?.url.searchParams.get("team") ?? selected?.id);
  let comparing = $derived(!!data.myTeam && selected?.id !== data.myTeam.id);

  function gap(score: number) {
    const d = score - myScore;
    if (d === 0) return { text: "tied", tone: "text-text/40" };
    return d > 0 ? { text: `+${d} ahead`, tone: "text-wrong/80" } : { text: `${-d} behind`, tone: "text-correct/80" };
  }

  function pick(id: string) {
    goto(`/teams/scout?team=${id}`, { noScroll: true, keepFocus: true });
  }
</script>

<svelte:head><title>Scout teams - OpenCTF</title></svelte:head>

<div class="flex flex-col md:flex-row min-h-page md:h-page mt-nav bg-raised text-muted-text">
  <aside
    class="w-full md:w-[21.25rem] shrink-0 flex flex-col bg-elevated border-b md:border-b-0 md:border-r border-text/6"
  >
    <div class="px-5 pt-5 pb-3">
      <a
        href="/teams"
        class="inline-flex items-center gap-1.5 text-label text-text/35 no-underline transition-colors
          duration-100 hover:text-text"
      >
        <ArrowLeft size="0.75rem" />
        My team
      </a>
      <h1 class="m-0 mt-3 text-lg font-bold text-text">Scout teams</h1>
    </div>

    <div class="px-4 pb-3">
      <div class="relative">
        <Search size="0.8125rem" class="absolute left-3 top-1/2 -translate-y-1/2 text-text/30 pointer-events-none" />
        <input
          type="text"
          placeholder="Search teams..."
          bind:value={search}
          class="w-full box-border bg-background border border-text/10 rounded-md text-muted-text text-xs
            py-2 pl-8 pr-3 outline-none focus:border-accent/50"
        />
      </div>
    </div>

    <div class="max-h-64 md:max-h-none md:flex-1 overflow-y-auto border-t border-text/5">
      {#each shown as team (team.id)}
        {@const own = team.id === data.myTeam?.id}
        {@const g = gap(team.score)}
        <button
          onclick={() => pick(team.id)}
          class="w-full flex items-center gap-3 px-4 py-2.5 border-0 border-l-2 border-solid text-left cursor-pointer
            transition-colors duration-100
            {team.id === highlighted
            ? 'bg-accent/8 border-l-accent'
            : 'bg-transparent border-l-transparent hover:bg-text/3'}"
        >
          <span class="w-6 text-center text-label font-bold tabular-nums text-text/35">
            {team.rank}
          </span>
          <div class="avatar w-8 h-8 text-xs rounded-md">
            {initial(team.name)}
          </div>
          <div class="flex-1 min-w-0">
            <div class="text-body truncate {own ? 'text-accent' : 'text-muted-text'}">
              {team.name}
            </div>
            <div class="text-label {own ? 'text-text/35' : g.tone}">
              {own ? "your team" : g.text}
            </div>
          </div>
          <span class="text-xs font-bold text-pts tabular-nums">
            {team.score}
          </span>
        </button>
      {:else}
        <div class="text-center py-10 text-xs text-text/25">
          No teams match "{search}"
        </div>
      {/each}
    </div>
  </aside>

  <main class="flex-1 min-w-0 md:overflow-y-auto">
    {#if !selected}
      <div class="h-full flex items-center justify-center text-body text-text/30">
        Pick a team on the left to scout it.
      </div>
    {:else}
      <div class="max-w-[60rem] mx-auto p-4 sm:p-6 flex flex-col gap-4">
        <section class="card p-6 flex flex-wrap items-center gap-5">
          <div class="avatar w-16 h-16 text-2xl rounded-lg">
            {initial(selected.name)}
          </div>
          <div class="flex-1 min-w-60">
            <h2 class="m-0 text-2xl font-bold text-text truncate">
              {selected.name}
            </h2>
            <div class="flex flex-wrap items-center gap-2 mt-2.5">
              <span class="pill text-text/70">
                <Trophy size="0.6875rem" class="text-pts" />
                Rank #{selected.rank}
              </span>
              <span class="pill font-bold text-pts bg-pts/8 border-pts/20 tabular-nums">
                {selected.score} pts
              </span>
              <span class="pill text-text/60">
                <Users size="0.6875rem" />
                {selected.member_count}
                {selected.member_count === 1 ? "member" : "members"}
              </span>
            </div>
          </div>
          {#if comparing}
            {@const d = selected.score - myScore}
            <div class="text-right">
              <div class="text-2xs tracking-[0.14em] text-text/35">VS YOUR TEAM</div>
              <div
                class="text-2xl font-bold tabular-nums mt-1 {d > 0
                  ? 'text-wrong'
                  : d < 0
                    ? 'text-correct'
                    : 'text-text'}"
              >
                {Math.abs(d)}
              </div>
              <div class="text-label text-text/35">
                {d > 0 ? "points ahead of you" : d < 0 ? "points behind you" : "dead even"}
              </div>
            </div>
          {/if}
        </section>

        {#await data.scouted}
          <div class="card py-12 text-center text-xs text-text/30">Loading solves...</div>
        {:then loaded}
          {@const categories = loaded?.comparison?.categories ?? []}
          {@const active = categories.filter((c) => c.them > 0 || c.us > 0)}
          {@const untouched = categories
            .filter((c) => !c.them && !c.us)
            .map((c) => fmt(c.category))
            .join(", ")}
          {@const untouchedCount = categories.length - active.length}
          <div class="grid grid-cols-1 lg:grid-cols-2 gap-4 items-start">
            <section class="card p-5">
              <div class="flex items-center justify-between mb-1">
                <div class="card-title">By category</div>
                {#if comparing}
                  <div class="flex items-center gap-3 text-2xs text-text/40">
                    <span class="inline-flex items-center gap-1.5">
                      <span class="w-2.5 h-1 rounded-full bg-accent"></span>them
                    </span>
                    <span class="inline-flex items-center gap-1.5">
                      <span class="w-2.5 h-1 rounded-full bg-text/40"></span>you
                    </span>
                  </div>
                {/if}
              </div>
              <div class="flex flex-col gap-0.5 mt-2 -mx-2">
                {#each active as { category, total, them, us } (category)}
                  {@const Icon = CAT_ICON[category]}
                  <a
                    href="/challenges?cat={category}"
                    title="Open {fmt(category)} challenges"
                    class="group block no-underline rounded-md px-2 py-1.5 border border-transparent
                      transition-colors duration-100 hover:bg-text/4 hover:border-text/10"
                  >
                    <div class="flex items-center gap-1.5 text-label mb-1">
                      <span style="color: var(--cat-{category}, var(--muted-text))" class="flex">
                        {#if Icon}<Icon size="0.6875rem" />{/if}
                      </span>
                      <span class="flex-1 group-hover:text-text {them > 0 ? 'text-text/75' : 'text-text/40'}">
                        {fmt(category)}
                      </span>
                      <span class="text-2xs text-text/40 tabular-nums">
                        {them}{comparing ? ` vs ${us}` : ""} / {total}
                      </span>
                      <ArrowRight
                        size="0.6875rem"
                        class="text-text/0 group-hover:text-text/60 transition-colors duration-100"
                      />
                    </div>
                    <div class="h-1 rounded-full bg-background overflow-hidden">
                      <div class="h-full rounded-full bg-accent" style="width: {(them / total) * 100}%"></div>
                    </div>
                    {#if comparing}
                      <div class="h-0.5 rounded-full bg-background overflow-hidden mt-0.5">
                        <div class="h-full rounded-full bg-text/40" style="width: {(us / total) * 100}%"></div>
                      </div>
                    {/if}
                  </a>
                {:else}
                  <div class="text-xs text-text/25 py-4 text-center">No solves in any category yet</div>
                {/each}
              </div>
              {#if untouchedCount > 0}
                <div class="mt-4 pt-3 border-t border-text/5 text-label text-text/30" title={untouched}>
                  {untouchedCount} untouched {untouchedCount === 1 ? "category" : "categories"}: {untouched}
                </div>
              {/if}
            </section>

            {#if comparing}
              <section class="card overflow-hidden">
                <div class="flex items-center justify-between px-5 pt-4 pb-3">
                  <div class="card-title">Solved by them</div>
                  <a
                    href="/challenges"
                    class="inline-flex items-center gap-1 text-label text-text/35 no-underline
                      transition-colors duration-100 hover:text-accent"
                  >
                    Challenges
                    <ArrowRight size="0.6875rem" />
                  </a>
                </div>
                {#each loaded?.comparison?.missing ?? [] as c (c.id)}
                  {@const Icon = CAT_ICON[c.category]}
                  <a
                    href="/challenges?open={c.id}"
                    title="Open {c.title}"
                    class="group flex items-center gap-3 px-5 py-2.5 border-t border-text/5 no-underline
                      transition-colors duration-100 hover:bg-text/4"
                  >
                    <span
                      class="flex items-center justify-center w-7 h-7 rounded-md shrink-0 cat-{c.category}"
                      title={fmt(c.category)}
                    >
                      {#if Icon}<Icon size="0.8125rem" />{/if}
                    </span>
                    <span class="flex-1 min-w-0 text-body text-muted-text truncate group-hover:text-text">
                      {c.title}
                    </span>
                    <span class="text-xs font-bold text-pts tabular-nums">
                      {c.points}
                    </span>
                    <ArrowRight
                      size="0.8125rem"
                      class="text-text/20 transition-all duration-100 group-hover:text-accent group-hover:translate-x-0.5"
                    />
                  </a>
                {:else}
                  <div class="text-center py-10 text-xs text-text/25 border-t border-text/5">
                    {categories.some((c) => c.them > 0)
                      ? "You've solved everything they have"
                      : "They haven't solved anything yet"}
                  </div>
                {/each}
              </section>
            {/if}
          </div>

          <Activity entries={loaded?.entries ?? []} error={loaded?.error ?? null} />
        {/await}
      </div>
    {/if}
  </main>
</div>
