<script lang="ts">
  import { invalidateAll, replaceState } from "$app/navigation";
  import { page } from "$app/state";
  import ChallengeModal from "#components/challenges/ChallengeModal.svelte";
  import ChallengeTable from "#components/challenges/ChallengeTable.svelte";
  import FlagField, { type FlagResult } from "#components/challenges/FlagField.svelte";
  import Sidebar from "#components/challenges/Sidebar.svelte";
  import StatCards from "#components/challenges/StatCards.svelte";
  import * as api from "#library/api.js";
  import { Search } from "lucide-svelte";
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  let search = $state("");
  let activeCat = $state(page.url.searchParams.get("cat") ?? "all");
  let openId = $state(page.url.searchParams.get("open"));

  let solved = $derived(new Set(data.solvedIds));
  let open = $derived(data.challenges.find((c) => c.id === openId));

  let matching = $derived(
    data.challenges
      .map((c, i) => ({ ...c, n: i + 1 }))
      .filter(
        (c) =>
          (activeCat === "all" || c.category === activeCat) && c.title.toLowerCase().includes(search.toLowerCase()),
      ),
  );

  async function submitFlag(flag: string, challengeId?: string): Promise<FlagResult> {
    try {
      const result = await api.submitFlag(flag, challengeId);
      if (!result.correct) return { ok: false, message: "Incorrect flag, try again." };
      await invalidateAll();
      return { ok: true, message: result.points ? `Correct! +${result.points} points` : "Already solved" };
    } catch (e) {
      return { ok: false, message: e instanceof Error ? e.message : "Submission failed." };
    }
  }

  function close() {
    openId = null;
    if (page.url.searchParams.has("open")) {
      const url = new URL(page.url.href);
      url.searchParams.delete("open");
      replaceState(url, {});
    }
  }
</script>

<svelte:head><title>Challenges - OpenCTF</title></svelte:head>

<div class="flex flex-col md:flex-row h-page mt-nav text-muted-text">
  <Sidebar challenges={data.challenges} {solved} bind:activeCat />

  <main class="flex-1 min-w-0 min-h-0 flex flex-col overflow-hidden bg-raised">
    <StatCards
      user={data.user}
      myTeam={data.myTeam}
      teams={data.teams}
      players={data.players}
      settings={data.settings}
      solvedCount={data.challenges.filter((c) => solved.has(c.id)).length}
      total={data.challenges.length}
    />

    <div class="flex flex-wrap items-center gap-3 px-4 pt-3 pb-1 shrink-0">
      <label
        class="flex items-center gap-2 w-full sm:w-55 h-[2.125rem] px-3 rounded-md bg-background border
          border-text/10 transition-colors duration-100 focus-within:border-accent/60 cursor-text"
      >
        <Search size="0.8125rem" class="shrink-0 text-text/30" />
        <input
          type="text"
          placeholder="Search challenges..."
          bind:value={search}
          class="flex-1 min-w-0 bg-transparent border-none outline-none p-0 text-xs text-muted-text"
        />
      </label>

      <FlagField label="Flag" placeholder="CTF&lbrace;submit_flag_here&rbrace;" onsubmit={(flag) => submitFlag(flag)} />
    </div>

    <ChallengeTable rows={matching} {solved} onselect={(c) => (openId = c.id)} />
  </main>
</div>

{#if open}
  {#key open.id}
    <ChallengeModal
      challenge={open}
      solved={solved.has(open.id)}
      onclose={close}
      onsubmit={(flag) => submitFlag(flag, open?.id)}
    />
  {/key}
{/if}
