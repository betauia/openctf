<script lang="ts">
  import { browser } from "$app/environment";
  import { invalidateAll, replaceState } from "$app/navigation";
  import { page } from "$app/state";
  import ChallengeModal from "@components/challenges/ChallengeModal.svelte";
  import ChallengeTable from "@components/challenges/ChallengeTable.svelte";
  import FlagField, { type FlagResult } from "@components/challenges/FlagField.svelte";
  import Sidebar from "@components/challenges/Sidebar.svelte";
  import StatCards from "@components/challenges/StatCards.svelte";
  import * as api from "@library/api";
  import { Search } from "lucide-svelte";
  import type { Challenge } from "@library/types";
  import type { PageProps } from "./$types";

  const COLLAPSE_KEY = "openctf_solved_collapsed";

  let { data }: PageProps = $props();

  let search = $state("");
  let activeCat = $state(page.url.searchParams.get("cat") ?? "all");
  let collapsed = $state(browser && localStorage.getItem(COLLAPSE_KEY) === "true");
  let openId = $state(page.url.searchParams.get("open"));

  $effect(() => {
    localStorage.setItem(COLLAPSE_KEY, String(collapsed));
  });

  let challenges = $derived(data.challenges);
  let solved = $derived(new Set(data.solvedIds));

  let open = $derived(challenges.find((c) => c.id === openId) ?? null);
  let solvedCount = $derived(challenges.filter((c) => solved.has(c.id)).length);

  let matching = $derived(
    challenges
      .map((c, i) => ({ ...c, n: i + 1 }))
      .filter(
        (c) =>
          (activeCat === "all" || c.category === activeCat) && c.title.toLowerCase().includes(search.toLowerCase()),
      ),
  );

  /** A correct flag reloads the page data, so solves and scores come from the backend. */
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
</script>

<svelte:head><title>Challenges - OpenCTF</title></svelte:head>

<div class="flex flex-col md:flex-row h-[calc(100vh-3rem)] mt-12 text-muted-text">
  <Sidebar challenges={data.challenges} {solved} bind:activeCat />

  <main class="flex-1 min-w-0 min-h-0 flex flex-col overflow-hidden bg-raised">
    <StatCards
      user={data.user}
      myTeam={data.myTeam}
      teams={data.teams}
      players={data.players}
      settings={data.settings}
      {solvedCount}
      total={challenges.length}
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
          class="flex-1 min-w-0 bg-transparent border-none outline-none p-0 text-[0.75rem] text-muted-text"
        />
      </label>

      <FlagField label="Flag" placeholder="CTF&lbrace;submit_flag_here&rbrace;" onsubmit={(flag) => submitFlag(flag)} />
    </div>

    <ChallengeTable
      unsolved={matching.filter((c) => !solved.has(c.id))}
      solved={matching.filter((c) => solved.has(c.id))}
      bind:collapsed
      onselect={(c) => (openId = c.id)}
    />
  </main>
</div>

{#if open}
  <ChallengeModal
    challenge={open}
    solved={solved.has(open.id)}
    onclose={() => {
      openId = null;
      if (page.url.searchParams.has("open")) {
        const url = new URL(page.url);
        url.searchParams.delete("open");
        replaceState(url, {});
      }
    }}
    onsubmit={(flag) => submitFlag(flag, open?.id)}
  />
{/if}
