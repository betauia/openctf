<script lang="ts">
  import { browser } from "$app/environment";
  import { invalidateAll, replaceState } from "$app/navigation";
  import { page } from "$app/state";
  import ChallengeModal from "@components/challenges/ChallengeModal.svelte";
  import ChallengeTable from "@components/challenges/ChallengeTable.svelte";
  import Sidebar from "@components/challenges/Sidebar.svelte";
  import * as api from "@library/api";
  import { ArrowRight } from "lucide-svelte";
  import type { Challenge } from "@library/types";
  import type { PageProps } from "./$types";

  const COLLAPSE_KEY = "openctf_solved_collapsed";

  let { data }: PageProps = $props();

  let search = $state("");
  let activeCat = $state(page.url.searchParams.get("cat") ?? "all");
  let collapsed = $state(browser && localStorage.getItem(COLLAPSE_KEY) === "true");
  let open = $state<Challenge | null>(data.challenges.find((c) => c.id === page.url.searchParams.get("open")) ?? null);
  let flagValue = $state("");
  let flagMessage = $state("");
  let flagOk = $state<boolean | null>(null);

  $effect(() => {
    localStorage.setItem(COLLAPSE_KEY, String(collapsed));
  });

  let challenges = $state(data.challenges);
  let solved = $state(new Set(data.solvedIds));

  $effect(() => {
    challenges = data.challenges;
    solved = new Set(data.solvedIds);
  });

  let matching = $derived(
    challenges
      .map((c, i) => ({ ...c, n: i + 1 }))
      .filter(
        (c) =>
          (activeCat === "all" || c.category === activeCat) && c.title.toLowerCase().includes(search.toLowerCase()),
      ),
  );

  /** A correct flag reloads the page data, so solves and scores come from the backend. */
  async function submitFlag(flag: string, challengeId?: string) {
    flagMessage = "";
    flagOk = null;
    try {
      const result = await api.submitFlag(flag, challengeId);
      flagOk = result.correct;
      flagMessage = result.correct ? `Correct! +${result.points} points` : "Incorrect flag, try again.";
      if (result.correct) {
        flagValue = "";
        await invalidateAll();
      }
    } catch (e) {
      flagOk = false;
      flagMessage = e instanceof Error ? e.message : "Submission failed.";
    }
  }
</script>

<svelte:head><title>Challenges - OpenCTF</title></svelte:head>

<div class="flex flex-col md:flex-row h-[calc(100vh-3rem)] mt-12 text-muted-text">
  <Sidebar challenges={data.challenges} {solved} bind:activeCat />

  <main class="flex-1 min-w-0 min-h-0 flex flex-col overflow-hidden bg-raised">
    <div class="grid grid-cols-3 lg:grid-cols-6 gap-px bg-text/5 border-b border-text/6 shrink-0">
      {#each [{ label: "Challenges", value: challenges.length }, { label: "Personal score", value: data.user.score }, { label: "Personal rank", value: data.user.rank }, { label: "Team", value: data.myTeam?.name ?? null }, { label: "Team score", value: data.myTeam?.score ?? null }, { label: "Team rank", value: data.myTeam?.rank ?? null }] as { label, value } (label)}
        <div class="min-w-0 bg-elevated py-3 px-3 flex flex-col items-center gap-1">
          <span
            class="max-w-full truncate whitespace-nowrap text-[0.625rem] sm:text-[0.6875rem] tracking-[0.12em] text-text/55"
          >
            {label}
          </span>
          <span class="max-w-full truncate text-[1rem] sm:text-[1.125rem] font-bold text-text">{value ?? "—"}</span>
        </div>
      {/each}
    </div>

    <div class="flex flex-wrap items-center gap-3 py-2.5 px-4 bg-elevated border-b border-text/6 shrink-0">
      <input
        type="text"
        placeholder="Search challenges..."
        bind:value={search}
        class="bg-background border border-text/10 rounded-md text-muted-text py-1.5 px-3 text-[0.75rem]
          outline-none w-full sm:w-55 transition-colors duration-100 focus:border-accent/60"
      />

      <form
        onsubmit={(e) => {
          e.preventDefault();
          if (flagValue.trim()) submitFlag(flagValue.trim());
        }}
        data-form-type="other"
        class="flex m-0 flex-1 min-w-0 sm:flex-none"
      >
        <input
          type="text"
          placeholder="CTF&lbrace;submit_flag_here&rbrace;"
          autocomplete="off"
          data-lpignore="true"
          bind:value={flagValue}
          class="bg-background border border-text/10 border-r-0 rounded-l-md text-muted-text py-1.5 px-3
            font-mono text-[0.75rem] outline-none flex-1 min-w-0 sm:flex-none sm:w-65 transition-colors
            duration-100 focus:border-accent/60"
        />
        <button
          type="submit"
          class="inline-flex items-center gap-1.5 border-none rounded-r-md text-white py-1.5 pl-3.5 pr-3
            text-[0.75rem] font-semibold cursor-pointer transition-colors duration-150
            {flagOk === null ? 'bg-accent hover:bg-accent-dark' : flagOk ? 'bg-correct-dark' : 'bg-wrong-dark'}"
        >
          Submit
          <ArrowRight size="0.8125rem" />
        </button>
      </form>

      <span class="text-[0.6875rem]" class:text-correct={flagOk === true} class:text-wrong={flagOk === false}>
        {open ? "" : flagMessage}
      </span>
    </div>

    <ChallengeTable
      unsolved={matching.filter((c) => !solved.has(c.id))}
      solved={matching.filter((c) => solved.has(c.id))}
      bind:collapsed
      onselect={(c) => (open = c)}
    />
  </main>
</div>

{#if open}
  <ChallengeModal
    challenge={open}
    solved={solved.has(open.id)}
    message={flagMessage}
    ok={flagOk}
    onclose={() => {
      open = null;
      if (page.url.searchParams.has("open")) {
        const url = new URL(page.url);
        url.searchParams.delete("open");
        replaceState(url, {});
      }
    }}
    onsubmit={(flag) => submitFlag(flag, open?.id)}
  />
{/if}
