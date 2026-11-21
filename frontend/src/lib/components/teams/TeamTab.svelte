<script lang="ts">
  import CopyButton from "@components/CopyButton.svelte";
  import Activity from "@components/teams/Activity.svelte";
  import MemberTable from "@components/teams/MemberTable.svelte";
  import type { ActivityEntry, Member, MyTeam, User } from "@library/types";
  import { RefreshCw } from "lucide-svelte";

  let {
    myTeam,
    me,
    activity,
    activityError,
    error,
    onleave,
    oncreate,
    onjoin,
    onmember,
    onrename,
    onregenerate,
    onkick,
    oncaptain,
  }: {
    myTeam: MyTeam | null;
    me: User;
    activity: ActivityEntry[];
    activityError: string | null;
    error: string;
    onleave: () => void;
    oncreate: (name: string) => void;
    onjoin: (inviteCode: string) => void;
    onmember: (member: Member) => void;
    onrename: (name: string) => void;
    onregenerate: () => void;
    onkick: (member: Member) => void;
    oncaptain: (member: Member) => void;
  } = $props();

  let newName = $state("");
  let inviteCode = $state("");
  let renaming = $state(false);
  let renameTo = $state("");

  // The backend's policy engine has the final say; this only decides what to show.
  let isCaptain = $derived(myTeam?.captain_id === me.id);

  let sortedMembers = $derived([...(myTeam?.members ?? [])].sort((a, b) => b.score - a.score));
</script>

<div class="flex-1 overflow-y-auto flex flex-col">
  {#if myTeam}
    <div class="flex flex-col shrink-0">
      <div class="flex items-center justify-between py-4.5 px-6 bg-elevated border-b border-border-subtle shrink-0">
        <div>
          {#if renaming}
            <form
              onsubmit={(e) => {
                e.preventDefault();
                onrename(renameTo);
                renaming = false;
              }}
              class="flex items-center gap-2 mb-1"
            >
              <input
                bind:value={renameTo}
                autocomplete="off"
                class="bg-background border border-border-subtle text-text text-xl font-bold py-0.5 px-2
                  outline-none focus:border-accent/50"
              />
              <button type="submit" class="text-[11px] text-accent bg-transparent border-none cursor-pointer">
                Save
              </button>
              <button
                type="button"
                onclick={() => (renaming = false)}
                class="text-[11px] text-text/30 bg-transparent border-none cursor-pointer"
              >
                Cancel
              </button>
            </form>
          {:else}
            <div class="flex items-center gap-2 mb-1">
              <div class="text-xl font-bold text-text">{myTeam.name}</div>
              {#if isCaptain}
                <button
                  onclick={() => {
                    renameTo = myTeam.name;
                    renaming = true;
                  }}
                  class="text-[11px] text-text/30 bg-transparent border-none cursor-pointer hover:text-accent"
                >
                  Rename
                </button>
              {/if}
            </div>
          {/if}
          <div class="flex items-center gap-3.5">
            <span class="text-[12px] font-semibold text-pts">
              {myTeam.score === undefined ? "—" : `${myTeam.score} pts`}
            </span>
            {#if myTeam.rank}
              <span class="text-[11px] text-text/30 tracking-[0.06em]">Rank #{myTeam.rank}</span>
            {/if}
          </div>
        </div>

        <div class="flex items-center gap-5">
          {#if myTeam.invite_code}
            <div class="flex items-center gap-2.5 bg-background border border-border-subtle py-1.5 px-3">
              <span class="font-mono text-[13px] text-muted-text tracking-[0.06em]">
                {myTeam.invite_code}
              </span>
              <CopyButton
                text={myTeam.invite_code}
                title="Copy invite code"
                class="flex items-center justify-center w-3.5 h-3.5 bg-transparent border-none text-text/30
                  cursor-pointer p-0 transition-colors duration-100 shrink-0 hover:text-muted-text"
              />
              {#if isCaptain}
                <button
                  onclick={onregenerate}
                  title="Generate a new invite code"
                  aria-label="Generate a new invite code"
                  class="flex items-center justify-center bg-transparent border-none text-text/30
                    cursor-pointer p-0 shrink-0 transition-colors duration-100 hover:text-muted-text"
                >
                  <RefreshCw size={13} />
                </button>
              {/if}
            </div>
          {/if}
          <button
            onclick={onleave}
            class="bg-accent border-none text-text text-[11px] tracking-[0.08em] py-1.75 px-4 cursor-pointer
              transition-colors duration-100 hover:bg-accent-dark"
          >
            Leave Team
          </button>
        </div>
      </div>

      <MemberTable
        members={sortedMembers}
        teamScore={myTeam.score ?? null}
        meId={me.id}
        onselect={onmember}
        onkick={isCaptain ? onkick : undefined}
        oncaptain={isCaptain ? oncaptain : undefined}
      />

      <div class="mt-1 text-[11px] text-wrong min-h-4 px-4 pb-2">{error}</div>

      <Activity entries={activity} error={activityError} />
    </div>
  {:else}
    <div class="flex-1 flex items-center justify-center px-6">
      <div class="flex gap-6">
        <div class="w-72 p-7 bg-elevated border border-text/6">
          <div class="text-[10px] tracking-[0.14em] text-text/30 mb-1">CREATE</div>
          <div class="text-[13px] text-text/60 mb-5">Start a new team and invite others.</div>
          <form
            onsubmit={(e) => {
              e.preventDefault();
              oncreate(newName);
            }}
            class="flex flex-col gap-2.5"
          >
            <input type="text" placeholder="Team name" autocomplete="off" bind:value={newName} class="field" />
            <button type="submit" class="btn-primary">Create</button>
            <div class="text-[11px] text-wrong min-h-4">{error}</div>
          </form>
        </div>

        <div class="flex flex-col items-center justify-center gap-2">
          <div class="w-px h-12 bg-text/8"></div>
          <span class="text-[10px] text-text/20 tracking-[0.1em]">OR</span>
          <div class="w-px h-12 bg-text/8"></div>
        </div>

        <div class="w-72 p-7 bg-elevated border border-text/6">
          <div class="text-[10px] tracking-[0.14em] text-text/30 mb-1">JOIN</div>
          <div class="text-[13px] text-text/60 mb-5">Enter an invite code from a teammate.</div>
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
    </div>
  {/if}
</div>
