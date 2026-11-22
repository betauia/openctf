<script lang="ts">
  import CopyButton from "@components/CopyButton.svelte";
  import FlagField, { type FlagResult } from "@components/challenges/FlagField.svelte";
  import { untrack } from "svelte";
  import * as api from "@library/api";
  import { CAT_ICON } from "@library/categories";
  import type { Challenge, ChallengeFile, Solve } from "@library/types";
  import { fmt, initial, timeAgo } from "@library/utils";
  import {
    ArrowRight,
    Check,
    ChevronDown,
    ChevronUp,
    Download,
    File,
    Flag,
    Play,
    Square,
    User,
    Users,
    X,
  } from "lucide-svelte";

  let {
    challenge,
    solved,
    onclose,
    onsubmit,
  }: {
    challenge: Challenge;
    solved: boolean;
    onclose: () => void;
    onsubmit: (flag: string) => Promise<FlagResult>;
  } = $props();

  let tab = $state<"overview" | "solves">("overview");
  let descExpanded = $state(false);

  let instanceId = $state<string | null>(null);
  let conn = $state<{ host: string; port: string } | null>(null);
  let connBusy = $state(false);
  let connError = $state("");

  let solves = $state<Solve[] | null>(null);
  let files = $state<ChallengeFile[]>([]);
  let fileError = $state("");

  $effect(() => {
    const id = challenge.id;
    files = [];
    fileError = "";
    api
      .challengeFiles(id)
      .then((f) => (files = f))
      .catch(() => (files = []));
  });

  async function download(file: ChallengeFile) {
    fileError = "";
    try {
      await api.downloadFile(challenge.id, file);
    } catch (e) {
      fileError = e instanceof Error ? e.message : "Download failed.";
    }
  }
  let solvesError = $state("");

  // Reset per-challenge state whenever a different challenge is opened.
  $effect(() => {
    challenge.id;
    tab = "overview";
    descExpanded = false;
    instanceId = null;
    conn = null;
    connError = "";
    solves = null;
    solvesError = "";
  });

  let description = $derived(challenge.description ?? "No description provided.");
  let descLong = $derived(description.length > 200 || description.split("\n").length > 4);
  let collapsed = $derived(descLong && !descExpanded);
  let catColor = $derived(`var(--cat-${challenge.category}, var(--muted-text))`);
  let diffColor = $derived(`var(--diff-${challenge.difficulty.toLowerCase()}, var(--muted-text))`);

  function openSolves() {
    tab = "solves";
    loadSolves();
  }

  $effect(() => {
    challenge.solves;
    if (untrack(() => tab) === "solves") untrack(loadSolves);
  });
  let CatIcon = $derived(CAT_ICON[challenge.category]);

  async function loadSolves() {
    solves = null;
    solvesError = "";
    try {
      solves = await api.challengeSolves(challenge.id);
    } catch (e) {
      solvesError = e instanceof Error ? e.message : "Failed to load.";
    }
  }

  function parseConnection(info: string) {
    const parts = info.trim().split(/\s+/);
    const port = parts.findLast((p) => /^\d+$/.test(p)) ?? "—";
    const host = parts.findLast((p) => p !== port && !/^(nc|ncat|telnet)$/i.test(p)) ?? info;
    return { host, port };
  }

  async function startInstance() {
    connError = "";
    if (!challenge.docker_image) {
      conn = parseConnection(challenge.connection_info ?? "");
      return;
    }
    connBusy = true;
    try {
      const instance = await api.createInstance(challenge.id);
      instanceId = instance.id;
      conn = { host: instance.host_ip ?? "—", port: instance.host_port ?? "—" };
    } catch (e) {
      connError = e instanceof Error ? e.message : "Could not start an instance.";
    }
    connBusy = false;
  }

  async function stopInstance() {
    connError = "";
    try {
      if (instanceId) await api.stopInstance(instanceId);
      instanceId = null;
      conn = null;
    } catch (e) {
      connError = e instanceof Error ? e.message : "Could not stop the instance.";
    }
  }
</script>

<svelte:window onkeydown={(e) => e.key === "Escape" && onclose()} />

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div
  class="fixed inset-0 z-100 flex items-center justify-center bg-black/65"
  onclick={(e) => e.target === e.currentTarget && onclose()}
>
  <div
    class="flex w-[34rem] max-w-[96vw] max-h-[90vh] flex-col overflow-hidden rounded-xl bg-raised border border-text/8"
  >
    <div class="shrink-0 px-6 pt-5 pb-4 bg-elevated border-b border-text/6">
      <div class="flex items-center gap-2">
        <span
          class="inline-flex items-center gap-1.5 rounded-full py-1 px-2.5 text-[0.6875rem] font-medium border"
          style="color:{catColor};
            background:color-mix(in srgb, {catColor} 12%, transparent);
            border-color:color-mix(in srgb, {catColor} 28%, transparent)"
        >
          {#if CatIcon}<CatIcon size="0.75rem" />{/if}
          {fmt(challenge.category)}
        </span>
        {#if solved}
          <span
            class="inline-flex items-center gap-1 rounded-full py-1 px-2.5 text-[0.6875rem] font-medium
              text-correct bg-correct/10 border border-correct/25"
          >
            <Check size="0.75rem" />
            Solved
          </span>
        {/if}
        <button
          onclick={onclose}
          aria-label="Close"
          class="ml-auto flex items-center justify-center w-7 h-7 rounded-md border-none bg-transparent
            text-text/30 cursor-pointer transition-colors duration-100 hover:text-text hover:bg-text/6"
        >
          <X size="1rem" />
        </button>
      </div>

      <h2 class="m-0 mt-3 text-xl font-bold leading-tight text-text">{challenge.title}</h2>

      <div class="flex flex-wrap items-center gap-2 mt-3">
        <span
          class="inline-flex items-center text-[0.6875rem] font-bold text-pts bg-pts/8 border border-pts/20
            rounded-full py-1 px-2.5 tabular-nums"
        >
          {challenge.points} pts
        </span>
        <span
          class="inline-flex items-center rounded-full py-1 px-2.5 text-[0.6875rem] font-medium border"
          style="color:{diffColor};
            background:color-mix(in srgb, {diffColor} 10%, transparent);
            border-color:color-mix(in srgb, {diffColor} 25%, transparent)"
        >
          {fmt(challenge.difficulty)}
        </span>
        {#if challenge.author}
          <span
            class="inline-flex items-center gap-1.5 text-[0.6875rem] text-text/60 bg-text/5 border border-text/8
              rounded-full py-1 px-2.5"
          >
            <User size="0.6875rem" />
            {challenge.author}
          </span>
        {/if}
        <button
          onclick={openSolves}
          class="inline-flex items-center gap-1.5 text-[0.6875rem] text-text/60 bg-text/5 border border-text/8
            rounded-full py-1 px-2.5 cursor-pointer transition-colors duration-100 hover:text-text
            hover:border-text/20"
        >
          <Users size="0.6875rem" />
          {challenge.solves}
          {challenge.solves === 1 ? "solve" : "solves"}
        </button>
      </div>

      <div class="flex items-center gap-1 mt-4">
        {#each [{ id: "overview", label: "Overview" }, { id: "solves", label: "Solves" }] as const as t (t.id)}
          <button
            onclick={() => (t.id === "solves" ? openSolves() : (tab = "overview"))}
            class="flex items-center px-3 py-1 rounded-full border-none text-[0.6875rem] tracking-[0.08em]
              cursor-pointer transition-colors duration-100
              {tab === t.id ? 'text-text bg-accent/15' : 'text-text/40 bg-transparent hover:text-text/70'}"
          >
            {t.label}
          </button>
        {/each}
      </div>
    </div>

    {#if tab === "overview"}
      <div class="flex-1 flex flex-col gap-4 overflow-y-auto p-6">
        <div class="relative">
          <p
            class="m-0 whitespace-pre-wrap text-[0.8125rem] leading-[1.75] text-text/70"
            class:max-h-23={collapsed}
            class:overflow-hidden={collapsed}
          >
            {description}
          </p>
          {#if collapsed}
            <div
              class="absolute right-0 bottom-5.5 left-0 h-9.5 pointer-events-none
                bg-[linear-gradient(transparent,var(--raised-color))]"
            ></div>
          {/if}
          {#if descLong}
            <button
              onclick={() => (descExpanded = !descExpanded)}
              class="inline-flex items-center gap-1 pt-1 px-0 pb-0 border-none bg-transparent text-[0.6875rem]
                tracking-[0.04em] cursor-pointer text-accent hover:underline"
            >
              {#if descExpanded}
                <ChevronUp size="1rem" /> Show less
              {:else}
                <ChevronDown size="1rem" /> Show more
              {/if}
            </button>
          {/if}
        </div>

        {#if solved}
          <div
            class="flex items-center gap-2 h-[2.5rem] px-3.5 rounded-md bg-correct/10 border border-correct/25
              text-[0.8125rem] text-correct"
          >
            <Check size="0.875rem" class="shrink-0" />
            Your team has solved this challenge
          </div>
        {:else}
          {#key challenge.id}
            <FlagField large label="Flag for {challenge.title}" placeholder="Submit flag & press enter" {onsubmit} />
          {/key}
        {/if}

        {#if challenge.connection_info || challenge.docker_image}
          <div class="overflow-hidden rounded-lg border border-text/6 bg-elevated">
            {#if conn}
              <div class="flex items-center gap-2 py-2.5 px-3.5 border-b border-correct/20 bg-correct/12">
                <span class="w-2 h-2 shrink-0 rounded-full bg-correct"></span>
                <span class="text-[0.625rem] font-bold tracking-widest text-correct">RUNNING</span>
              </div>
              <div class="flex items-center pr-3.5 border-b border-border-subtle">
                <div class="flex-1 p-3.5">
                  <div class="mb-1 font-mono text-[0.875rem] font-semibold text-muted-text">
                    {conn.host}
                  </div>
                  <div class="text-[0.5625rem] tracking-[0.12em] uppercase text-text/28">HOST</div>
                </div>
                <div class="flex-1 p-3.5">
                  <div class="mb-1 font-mono text-[0.875rem] font-semibold text-muted-text">
                    {conn.port}
                  </div>
                  <div class="text-[0.5625rem] tracking-[0.12em] uppercase text-text/28">PORT</div>
                </div>
                <div class="shrink-0 flex items-center gap-1.5 pr-3.5">
                  <CopyButton
                    text={instanceId ? `nc ${conn.host} ${conn.port}` : (challenge.connection_info ?? "")}
                    class="icon-btn"
                  />
                  <button
                    onclick={stopInstance}
                    title="Stop"
                    aria-label="Stop instance"
                    class="w-7 h-7 flex items-center justify-center rounded border border-wrong/30 text-[0.625rem]
                        cursor-pointer text-wrong bg-wrong/12 transition-colors duration-150 hover:bg-wrong/25"
                  >
                    <Square size="0.875rem" />
                  </button>
                </div>
              </div>
            {:else}
              <div class="flex items-center justify-between py-3 px-3.5">
                <div>
                  <div class="text-[0.8125rem] font-bold text-text">Create Instance</div>
                  <div class="mt-0.5 text-[0.6875rem] text-text/32">Creates for all team members</div>
                </div>
                <button
                  onclick={startInstance}
                  disabled={connBusy}
                  title="Start"
                  aria-label="Start instance"
                  class="shrink-0 w-7.5 h-7.5 flex items-center justify-center rounded-full
                      border border-text/10 text-[0.6875rem] cursor-pointer text-muted-text bg-text/8
                      transition-colors duration-150 hover:enabled:bg-text/14
                      disabled:opacity-50 disabled:cursor-default"
                >
                  <Play size="1rem" />
                </button>
              </div>
            {/if}

            {#if connError}
              <div class="py-2 px-3.5 text-[0.6875rem] text-wrong border-t border-border-subtle">
                {connError}
              </div>
            {/if}
          </div>
        {/if}

        {#if files.length > 0}
          <div class="overflow-hidden rounded-lg border border-text/6 bg-elevated">
            <div class="py-3 px-3.5">
              <div class="text-[0.8125rem] font-semibold text-muted-text">Files</div>
              <div class="mt-0.5 text-[0.6875rem] text-text/32">Required files for this challenge</div>
            </div>
            <div class="flex flex-col gap-1 px-3.5 pb-3">
              {#each files as file (file.id)}
                <button
                  onclick={() => download(file)}
                  class="group flex items-center justify-between rounded py-1.75 px-2.5 text-xs cursor-pointer
                      border border-border-subtle bg-text/4 text-text/60 transition-colors duration-150
                      hover:bg-text/8 hover:text-text/90"
                >
                  <span class="inline-flex items-center gap-1.5 font-mono">
                    <File size="0.8125rem" />
                    {file.filename}
                  </span>
                  <span class="flex items-center text-text/30 transition-colors duration-150 group-hover:text-text/70">
                    <Download size="0.8125rem" />
                  </span>
                </button>
              {/each}
              {#if fileError}
                <div class="text-[0.6875rem] text-wrong">{fileError}</div>
              {/if}
            </div>
          </div>
        {/if}
      </div>
    {:else}
      <div class="flex-1 flex flex-col overflow-y-auto p-6">
        {#if solvesError}
          <div class="text-center py-12 text-[0.8125rem] text-text/25">{solvesError}</div>
        {:else if !solves}
          <div class="text-center py-12 text-[0.8125rem] text-text/25">Loading...</div>
        {:else if solves.length === 0}
          <div class="text-center py-12 text-[0.8125rem] text-text/25">No solves yet. Be the first.</div>
        {:else}
          <div class="rounded-lg bg-elevated border border-text/6 overflow-hidden">
            {#each solves as solve, i (solve.uid)}
              <div class="flex items-center gap-3 py-2.5 px-4 border-t border-text/5 first:border-t-0">
                <span class="w-5 text-[0.6875rem] tabular-nums {i === 0 ? 'text-pts font-bold' : 'text-text/25'}">
                  {i + 1}
                </span>
                <div class="avatar w-7 h-7 text-[0.6875rem] rounded">{initial(solve.username)}</div>
                <div class="flex-1 min-w-0 flex items-center gap-2">
                  <a
                    href="/teams?member={solve.uid}"
                    class="text-[0.8125rem] text-muted-text font-medium no-underline truncate hover:text-accent"
                  >
                    {solve.username}
                  </a>
                  {#if solve.team && solve.team_id}
                    <a
                      href="/teams/scout?team={solve.team_id}"
                      title="Inspect {solve.team}"
                      class="inline-flex items-center gap-1 text-[0.6875rem] text-text/45 no-underline rounded-full
                        border border-text/10 bg-text/4 py-px px-2 transition-colors duration-100
                        hover:text-text hover:border-text/25"
                    >
                      <Users size="0.625rem" />
                      {solve.team}
                    </a>
                  {/if}
                </div>
                <span class="text-[0.6875rem] text-text/30 shrink-0">{timeAgo(solve.solved_at)}</span>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    {/if}
  </div>
</div>
