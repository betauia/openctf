<script lang="ts">
  import CopyButton from "#components/CopyButton.svelte";
  import Activity from "#components/teams/Activity.svelte";
  import MemberTable from "#components/teams/MemberTable.svelte";
  import { CAT_ICON } from "#library/categories.js";
  import type { ActivityEntry, Member, MyTeam, Progress, User } from "#library/types.js";
  import { fmt, initial } from "#library/utils.js";
  import { ArrowRight, Check, LogOut, Pencil, RefreshCw, Telescope, Trophy, Users } from "lucide-svelte";
  import { fly } from "svelte/transition";

  let {
    myTeam,
    me,
    activity,
    activityError,
    error,
    maxTeamSize,
    progress,
    oncreate,
    onjoin,
    onmember,
    onkick,
    oncaptain,
    onrename,
    onregenerate,
    onleave,
  }: {
    myTeam: MyTeam | null;
    me: User;
    activity: ActivityEntry[];
    activityError: string | null;
    error: string;
    maxTeamSize: number | null;
    progress: Progress | null;
    oncreate: (name: string) => void;
    onjoin: (inviteCode: string) => void;
    onmember: (member: Member) => void;
    onkick: (member: Member) => void;
    oncaptain: (member: Member) => void;
    onrename: (name: string) => void;
    onregenerate: () => Promise<boolean>;
    onleave: () => void;
  } = $props();

  let newName = $state("");
  let inviteCode = $state("");
  let renaming = $state(false);
  let renameTo = $state("");
  let regenerating = $state(false);
  let regenerated = $state(false);

  async function regenerate() {
    if (!confirm("Generate a new code? The old one stops working.")) return;
    regenerating = true;
    regenerated = await onregenerate();
    regenerating = false;
    setTimeout(() => (regenerated = false), 1200);
  }

  let isCaptain = $derived(myTeam?.captain_id === me.id);
  let openSlots = $derived(maxTeamSize && myTeam ? Math.max(0, maxTeamSize - myTeam.members.length) : null);

  let progressPct = $derived(progress?.total ? Math.round((progress.solved / progress.total) * 100) : 0);
</script>

<div class="flex-1 overflow-y-auto flex flex-col">
  {#if myTeam}
    <div class="w-full max-w-[75rem] mx-auto p-6 flex flex-col gap-4">
      <section class="card p-6 flex flex-wrap items-center gap-6">
        <div class="avatar w-16 h-16 text-2xl rounded-lg">
          {initial(myTeam.name)}
        </div>

        <div class="flex-1 min-w-60">
          {#if renaming}
            <form
              onsubmit={(e) => {
                e.preventDefault();
                renaming = false;
                if (renameTo.trim() && renameTo !== myTeam.name) onrename(renameTo.trim());
              }}
              class="flex items-center gap-2"
            >
              <input
                bind:value={renameTo}
                autocomplete="off"
                maxlength="64"
                onkeydown={(e) => e.key === "Escape" && (renaming = false)}
                class="bg-background border border-text/10 text-text text-xl font-bold py-1 px-2.5 outline-none
                  rounded focus:border-accent/50 w-72"
              />
              <button
                type="submit"
                class="border-none bg-accent text-white text-label font-bold tracking-[0.08em] py-2 px-3.5
                  rounded cursor-pointer hover:bg-accent-dark"
              >
                Save
              </button>
              <button
                type="button"
                onclick={() => (renaming = false)}
                class="bg-transparent border-none text-label text-text/40 cursor-pointer hover:text-text"
              >
                Cancel
              </button>
            </form>
          {:else}
            <div class="flex items-center gap-2">
              <h1 class="m-0 text-2xl font-bold text-text truncate">
                {myTeam.name}
              </h1>
              {#if isCaptain}
                <button
                  onclick={() => {
                    renameTo = myTeam.name;
                    renaming = true;
                  }}
                  title="Rename team"
                  aria-label="Rename team"
                  class="flex items-center justify-center w-7 h-7 rounded bg-transparent border-none text-text/30
                    cursor-pointer transition-colors duration-100 hover:text-text hover:bg-text/5"
                >
                  <Pencil size="0.8125rem" />
                </button>
              {/if}
            </div>
          {/if}

          <div class="flex flex-wrap items-center gap-2 mt-2.5">
            <a
              href="/scoreboard"
              title="View scoreboard"
              class="pill text-text/70 no-underline transition-colors duration-100 hover:text-text hover:border-text/20"
            >
              <Trophy size="0.6875rem" class="text-pts" />
              Rank #{myTeam.rank}
            </a>
            <span class="pill font-bold text-pts bg-pts/8 border-pts/20 tabular-nums">
              {myTeam.score} pts
            </span>
            <span class="pill text-text/60">
              <Users size="0.6875rem" />
              {myTeam.members.length}{maxTeamSize ? ` / ${maxTeamSize}` : ""} members
            </span>
            <a
              href="/teams/scout"
              class="pill text-text/70 no-underline transition-colors duration-100 hover:text-text hover:border-text/20"
            >
              <Telescope size="0.6875rem" />
              Scout teams
            </a>
            <button
              onclick={() => confirm(`Leave ${myTeam.name}?`) && onleave()}
              class="pill text-wrong bg-wrong/5 border-wrong/35 cursor-pointer transition-colors duration-100 hover:bg-wrong/10 hover:border-wrong/60"
            >
              <LogOut size="0.6875rem" />
              Leave team
            </button>
          </div>
        </div>

        {#if myTeam.invite_code}
          <div class="w-80 max-w-full rounded-md bg-background/70 border border-text/8 p-3.5">
            <div class="flex items-center justify-between mb-2">
              <span class="text-2xs tracking-[0.14em] text-text/40"> INVITE CODE </span>
              {#if openSlots !== null}
                <span class="text-2xs {openSlots === 0 ? 'text-wrong/80' : 'text-text/35'}">
                  {openSlots === 0 ? "Team is full" : `${openSlots} ${openSlots === 1 ? "slot" : "slots"} left`}
                </span>
              {/if}
            </div>
            <div class="flex items-center gap-2">
              <span class="relative flex-1 min-w-0 h-5 overflow-hidden">
                {#key myTeam.invite_code}
                  <span
                    in:fly={{ y: 14, duration: 260, delay: 60 }}
                    out:fly={{ y: -14, duration: 200 }}
                    class="absolute inset-0 font-mono text-body leading-5 tracking-[0.04em] truncate select-all
                      {regenerated ? 'text-correct' : 'text-text'} transition-colors duration-700"
                  >
                    {myTeam.invite_code}
                  </span>
                {/key}
              </span>
              <CopyButton
                text={myTeam.invite_code}
                title="Copy invite code"
                size="0.8125rem"
                class="flex items-center justify-center w-7 h-7 rounded bg-text/5 border-none text-text/50
                  cursor-pointer shrink-0 transition-colors duration-100 hover:text-text hover:bg-text/10"
              />
              {#if isCaptain}
                <button
                  onclick={regenerate}
                  disabled={regenerating}
                  title="Generate a new invite code"
                  aria-label="Generate a new invite code"
                  class="flex items-center justify-center w-7 h-7 rounded bg-text/5 border-none text-text/50
                    cursor-pointer shrink-0 transition-colors duration-100 hover:text-text hover:bg-text/10
                    disabled:cursor-default"
                >
                  {#if regenerated}
                    <Check size="0.8125rem" class="text-correct animate-[icon-pop_0.15s_ease-out]" />
                  {:else}
                    <RefreshCw size="0.8125rem" class={regenerating ? "animate-spin" : ""} />
                  {/if}
                </button>
              {/if}
            </div>
            <div class="text-label text-text/30 mt-2">Share it with teammates so they can join.</div>
          </div>
        {/if}
      </section>

      {#if error}
        <div class="text-xs text-wrong bg-wrong/8 border border-wrong/20 rounded-md py-2 px-3.5">
          {error}
        </div>
      {/if}

      <div class="grid grid-cols-1 lg:grid-cols-[minmax(0,1fr)_22.5rem] gap-4 items-start">
        <div class="flex flex-col gap-4 min-w-0">
          <MemberTable
            members={myTeam.members}
            teamScore={myTeam.score}
            meId={me.id}
            captainId={myTeam.captain_id}
            {maxTeamSize}
            onselect={onmember}
            onkick={isCaptain ? onkick : undefined}
            oncaptain={isCaptain ? oncaptain : undefined}
          />

          <Activity entries={activity} error={activityError} />
        </div>

        <div class="flex flex-col gap-4">
          {#if progress && progress.total > 0}
            <section class="card p-5">
              <div class="flex items-center justify-between mb-3">
                <div class="card-title">Progress</div>
                <a
                  href="/challenges"
                  class="inline-flex items-center gap-1 text-label text-text/35 no-underline
                    transition-colors duration-100 hover:text-accent"
                >
                  Challenges
                  <ArrowRight size="0.6875rem" />
                </a>
              </div>

              <div class="flex items-baseline gap-1.5 mb-2">
                <span class="text-2xl font-bold text-text tabular-nums">
                  {progress.solved}
                </span>
                <span class="text-xs text-text/35">
                  / {progress.total} solved
                </span>
                <span class="ml-auto text-label text-text/35 tabular-nums">
                  {progressPct}%
                </span>
              </div>
              <div class="h-1.5 rounded-full bg-background overflow-hidden mb-4">
                <div
                  class="h-full rounded-full bg-accent transition-[width] duration-300"
                  style="width: {progressPct}%"
                ></div>
              </div>

              <div class="grid grid-cols-2 gap-x-1 gap-y-0.5 -mx-2">
                {#each progress.categories as { category, total, solved } (category)}
                  {@const Icon = CAT_ICON[category]}
                  <a
                    href="/challenges?cat={category}"
                    title="Open {fmt(category)} challenges"
                    style="color: var(--cat-{category}, var(--muted-text))"
                    class="group min-w-0 block no-underline rounded-md px-2 py-1.5 border border-transparent
                      transition-colors duration-100 hover:bg-current/8 hover:border-current/25"
                  >
                    <div class="flex items-center gap-1.5 text-label mb-1">
                      {#if Icon}<Icon size="0.6875rem" />{/if}
                      <span
                        class="flex-1 truncate group-hover:text-text
                          {solved > 0 ? 'text-text/70' : 'text-text/40'}"
                      >
                        {fmt(category)}
                      </span>
                      <span class="text-2xs text-text/35 tabular-nums group-hover:hidden">
                        {solved}/{total}
                      </span>
                      <ArrowRight size="0.6875rem" class="hidden group-hover:block" />
                    </div>
                    <div class="h-0.75 rounded-full bg-background overflow-hidden">
                      <div class="h-full rounded-full bg-current" style="width: {(solved / total) * 100}%"></div>
                    </div>
                  </a>
                {/each}
              </div>
            </section>
          {/if}
        </div>
      </div>
    </div>
  {:else}
    <div class="flex-1 flex flex-col items-center justify-center gap-6 px-6">
      <div class="flex gap-6">
        <div class="w-72 p-7 card">
          <div class="text-2xs tracking-[0.14em] text-text/30 mb-1">CREATE</div>
          <div class="text-body text-text/60 mb-5">Start a new team and invite others.</div>
          <form
            onsubmit={(e) => {
              e.preventDefault();
              oncreate(newName);
            }}
            class="flex flex-col gap-2.5"
          >
            <input type="text" placeholder="Team name" autocomplete="off" bind:value={newName} class="field" />
            <button type="submit" class="btn-primary">Create</button>
            <div class="text-label text-wrong min-h-4">{error}</div>
          </form>
        </div>

        <div class="flex flex-col items-center justify-center gap-2">
          <div class="w-px h-12 bg-text/8"></div>
          <span class="text-2xs text-text/20 tracking-[0.1em]">OR</span>
          <div class="w-px h-12 bg-text/8"></div>
        </div>

        <div class="w-72 p-7 card">
          <div class="text-2xs tracking-[0.14em] text-text/30 mb-1">JOIN</div>
          <div class="text-body text-text/60 mb-5">Enter an invite code from a teammate.</div>
          <form
            onsubmit={(e) => {
              e.preventDefault();
              onjoin(inviteCode);
            }}
            class="flex flex-col gap-2.5"
          >
            <input type="text" placeholder="Invite code" autocomplete="off" bind:value={inviteCode} class="field" />
            <button type="submit" class="btn-primary">Join</button>
          </form>
        </div>
      </div>

      <a
        href="/teams/scout"
        class="inline-flex items-center gap-1.5 text-xs text-text/40 no-underline transition-colors
          duration-100 hover:text-text"
      >
        <Telescope size="0.8125rem" />
        Scout other teams first
        <ArrowRight size="0.75rem" />
      </a>
    </div>
  {/if}
</div>
