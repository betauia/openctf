<script lang="ts">
  import { browser } from "$app/environment";
  import { invalidateAll } from "$app/navigation";
  import ChallengeModal from "@components/challenges/ChallengeModal.svelte";
  import ChallengeTable from "@components/challenges/ChallengeTable.svelte";
  import Sidebar from "@components/challenges/Sidebar.svelte";
  import * as api from "@library/api";
  import type { Challenge } from "@library/types";
  import type { PageProps } from "./$types";

  const COLLAPSE_KEY = "openctf_solved_collapsed";

  let { data }: PageProps = $props();

  let search = $state("");
  let activeCat = $state("all");
  let collapsed = $state(browser && localStorage.getItem(COLLAPSE_KEY) === "true");
  let open = $state<Challenge | null>(null);
  let flagValue = $state("");
  let flagMessage = $state("");
  let flagOk = $state<boolean | null>(null);

  $effect(() => {
    localStorage.setItem(COLLAPSE_KEY, String(collapsed));
  });

  // Held as state, not derived, so a push channel can patch rows in place
  // instead of forcing a full reload. The effect re-seeds when the load re-runs.
  // svelte-ignore state_referenced_locally -- seeded on purpose, re-seeded below
  let challenges = $state(data.challenges);
  // svelte-ignore state_referenced_locally
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

<div class="flex h-[calc(100vh-48px)] mt-12 text-muted-text">
  <Sidebar challenges={data.challenges} {solved} bind:activeCat />

  <main class="flex-1 flex flex-col overflow-hidden bg-raised">
    <div class="flex border-b border-text/6 bg-elevated shrink-0">
      {#each [{ label: "Challenges", value: challenges.length }, { label: "Personal score", value: data.user.score }, { label: "Personal rank", value: data.user.rank }, { label: "Team", value: data.myTeam?.name ?? null }, { label: "Team score", value: data.myTeam?.score ?? null }, { label: "Team rank", value: data.myTeam?.rank ?? null }] as { label, value } (label)}
        <div class="flex-1 py-3 px-5 border-r border-text/5 flex flex-col items-center gap-1 last:border-r-0">
          <span class="text-[11px] tracking-[0.14em] text-text/55">{label}</span>
          <span class="text-[18px] font-bold text-text">{value ?? "—"}</span>
        </div>
      {/each}
    </div>

    <div class="flex items-center gap-3 py-2.5 px-4 bg-elevated border-b border-text/6 shrink-0">
      <input
        type="text"
        placeholder="Search challenges..."
        bind:value={search}
        class="bg-background border border-text/10 text-muted-text py-1.5 px-2.5 text-[12px]
          outline-none w-55 focus:border-accent"
      />

      <form
        onsubmit={(e) => {
          e.preventDefault();
          if (flagValue.trim()) submitFlag(flagValue.trim());
        }}
        data-form-type="other"
        class="flex m-0"
      >
        <input
          type="text"
          placeholder="CTF&lbrace;submit_flag_here&rbrace;"
          autocomplete="off"
          data-lpignore="true"
          bind:value={flagValue}
          class="bg-background border border-text/10 border-r-0 text-muted-text py-1.5 px-2.5 text-[12px]
            outline-none w-65 focus:border-accent/40"
        />
        <button
          type="submit"
          class="border-none text-white py-1.5 px-4 text-[11px] tracking-[0.1em] cursor-pointer font-bold
            transition-colors duration-150
            {flagOk === null ? 'bg-accent hover:bg-accent-dark' : flagOk ? 'bg-correct-dark' : 'bg-wrong-dark'}"
        >
          Submit
        </button>
      </form>

      <span class="text-[11px]" class:text-correct={flagOk === true} class:text-wrong={flagOk === false}>
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
    onclose={() => (open = null)}
    onsubmit={(flag) => submitFlag(flag, open?.id)}
  />
{/if}
