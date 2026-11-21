<script lang="ts">
  import CopyButton from "@components/CopyButton.svelte";
  import * as api from "@library/api";
  import { CAT_ICON } from "@library/categories";
  import type { Challenge, ChallengeFile, Solve } from "@library/types";
  import { fmt, pad2, timeAgo } from "@library/utils";
  import { ArrowRight, ChevronDown, ChevronUp, Download, File, Play, Square, X } from "lucide-svelte";

  let {
    challenge,
    solved,
    message,
    ok,
    onclose,
    onsubmit,
  }: {
    challenge: Challenge;
    solved: boolean;
    message: string;
    ok: boolean | null;
    onclose: () => void;
    onsubmit: (flag: string) => void;
  } = $props();

  let tab = $state<"overview" | "solves">("overview");
  let descExpanded = $state(false);
  let flagValue = $state("");

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
    flagValue = "";
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
  <div class="relative">
    {#if solved}
      <div
        class="absolute top-0 left-1/2 -translate-x-1/2 whitespace-nowrap pointer-events-none
          py-0.75 px-2.5 text-[0.8125rem] font-bold tracking-widest
          border border-correct/18 bg-correct/6 text-correct"
      >
        Solved
      </div>
    {/if}

    <div
      class="flex w-125 max-w-[96vw] max-h-[90vh] flex-col overflow-hidden bg-raised
        "
    >
      <div class="shrink-0 pt-4.5 px-5 pb-4 border-b border-border-subtle bg-elevated">
        <div class="flex items-center justify-between mb-2">
          <span class="text-[0.625rem] tracking-[0.14em] text-text/28">CHALLENGE NAME</span>
          <div class="flex items-center gap-2">
            <div
              class="w-9 h-9 flex items-center justify-center border"
              style="color:{catColor};
                background:color-mix(in srgb, {catColor} 12%, transparent);
                border-color:color-mix(in srgb, {catColor} 25%, transparent)"
            >
              {#if CatIcon}<CatIcon size="1.25rem" />{/if}
            </div>
            <button
              onclick={onclose}
              aria-label="Close"
              class="p-0 leading-none text-[0.8125rem] cursor-pointer text-text/22 hover:text-text/70"
            >
              <X size="1rem" />
            </button>
          </div>
        </div>
        <div class="text-xl font-bold tracking-[0.01em] leading-tight text-text/92">{challenge.title}</div>
      </div>

      <div class="flex shrink-0 border-b border-border-subtle bg-elevated">
        <button
          onclick={() => (tab = "overview")}
          class="flex-1 p-2.75 border-b-2 text-xs font-semibold tracking-wider cursor-pointer
            transition-colors duration-150 hover:text-text/60
            {tab === 'overview' ? 'text-muted-text border-b-accent' : 'text-text/32 border-transparent'}"
        >
          Overview
        </button>
        <button
          onclick={() => {
            tab = "solves";
            loadSolves();
          }}
          class="flex-1 p-2.75 border-b-2 text-xs font-semibold tracking-wider cursor-pointer
            transition-colors duration-150 hover:text-text/60
            {tab === 'solves' ? 'text-muted-text border-b-accent' : 'text-text/32 border-transparent'}"
        >
          Solves
        </button>
      </div>

      {#if tab === "overview"}
        <div class="flex-1 flex flex-col gap-3.5 overflow-y-auto p-5">
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
                class="inline-flex items-center gap-1 pt-1 px-0 pb-0 text-[0.6875rem] tracking-[0.04em]
                  cursor-pointer text-accent hover:underline"
              >
                {#if descExpanded}
                  <ChevronUp size="1rem" /> Show less
                {:else}
                  <ChevronDown size="1rem" /> Show more
                {/if}
              </button>
            {/if}
          </div>

          <form
            onsubmit={(e) => {
              e.preventDefault();
              if (flagValue.trim()) onsubmit(flagValue.trim());
            }}
            data-form-type="other"
            class="flex m-0 overflow-hidden rounded-md border border-text/12"
          >
            <input
              type="text"
              placeholder="Submit flag & press enter"
              autocomplete="off"
              data-lpignore="true"
              bind:value={flagValue}
              class="flex-1 min-w-0 border-none bg-background py-2.5 px-3.5 font-mono text-xs
                text-muted-text outline-none focus:bg-text/2"
            />
            <button
              type="submit"
              aria-label="Submit flag"
              class="shrink-0 border-none py-2.5 px-4.5 text-base cursor-pointer text-white
                transition-colors duration-150
                {ok === null ? 'bg-accent hover:bg-accent-dark' : ok ? 'bg-correct-dark' : 'bg-wrong-dark'}"
            >
              <ArrowRight size="1rem" />
            </button>
          </form>

          <div
            class="-mt-2 min-h-3.5 text-[0.6875rem]"
            class:text-correct={ok === true}
            class:text-wrong={ok === false}
          >
            {message}
          </div>

          {#if challenge.connection_info || challenge.docker_image}
            <div class="overflow-hidden rounded border border-border-subtle bg-elevated">
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
            <div class="overflow-hidden rounded border border-border-subtle bg-elevated">
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
                    <span
                      class="flex items-center text-text/30 transition-colors duration-150 group-hover:text-text/70"
                    >
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

          <div class="grid grid-cols-2 overflow-hidden rounded border border-border-subtle">
            {#each [{ value: challenge.points, color: "var(--pts-color)", label: "POINTS" }, { value: fmt(challenge.difficulty), color: `var(--diff-${challenge.difficulty.toLowerCase()}, var(--muted-text))`, label: "DIFFICULTY" }, { value: challenge.author ?? "—", color: "var(--muted-text)", label: "AUTHOR" }, { value: challenge.solves, color: "var(--muted-text)", label: "SOLVES" }] as { value, color, label } (label)}
              <div
                class="p-4 text-center bg-elevated border-r border-b border-border-subtle
                  [&:nth-child(even)]:border-r-0 [&:nth-last-child(-n+2)]:border-b-0"
              >
                <div class="mb-1 text-[1.0625rem] font-bold" style="color:{color}">{value}</div>
                <div class="text-[0.5625rem] tracking-[0.14em] uppercase text-text/28">{label}</div>
              </div>
            {/each}
          </div>
        </div>
      {:else}
        <div class="flex-1 flex flex-col overflow-y-auto p-5">
          {#if solvesError}
            <div class="text-center py-12 text-[0.8125rem] text-text/22">{solvesError}</div>
          {:else if !solves}
            <div class="text-center py-12 text-[0.8125rem] text-text/22">Loading...</div>
          {:else if solves.length === 0}
            <div class="text-center py-12 text-[0.8125rem] text-text/22">No solves yet.</div>
          {:else}
            {#each solves as solve, i (solve.uid)}
              <div class="flex items-center gap-3 py-2 px-1 border-b border-border-subtle last:border-b-0">
                <span class="text-[0.6875rem] text-text/18 w-6">{pad2(i + 1)}</span>
                <div class="flex-1 flex items-center gap-2">
                  <a
                    href="/teams?member={solve.uid}"
                    class="text-[0.8125rem] text-muted-text font-medium no-underline hover:text-accent"
                  >
                    {solve.username}
                  </a>
                  {#if solve.team}
                    <span class="text-[0.6875rem] text-text/30">{solve.team}</span>
                  {/if}
                </div>
                <span class="text-[0.6875rem] text-text/25">{timeAgo(solve.solved_at)}</span>
              </div>
            {/each}
          {/if}
        </div>
      {/if}
    </div>
  </div>
</div>
