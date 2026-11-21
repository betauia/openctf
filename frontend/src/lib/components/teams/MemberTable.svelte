<script lang="ts">
  import type { Member } from "@library/types";
  import { initial, pad2 } from "@library/utils";
  import { X } from "lucide-svelte";

  let {
    members,
    teamScore = null,
    meId,
    onselect,
    onkick,
    oncaptain,
  }: {
    members: Member[];
    /** Team total, for the contribution column. Omit to hide that column. */
    teamScore?: number | null;
    meId?: string;
    onselect?: (member: Member) => void;
    /** Captain-only actions. Passing either adds an actions column. */
    onkick?: (member: Member) => void;
    oncaptain?: (member: Member) => void;
  } = $props();

  let hasActions = $derived(Boolean(onkick || oncaptain));
</script>

<div class="flex-1 overflow-y-auto p-4 max-h-70 flex-none">
  <table class="w-full table-fixed border-separate border-spacing-x-0 border-spacing-y-0.5">
    <thead>
      <tr>
        <th class="th text-left w-9">#</th>
        <th class="th text-left w-2/5">Member</th>
        <th class="th text-center">Score</th>
        <th class="th text-center">Solves</th>
        {#if teamScore !== null}
          <th class="th text-left w-1/3">Contribution</th>
        {/if}
        {#if hasActions}
          <th class="th text-right w-28"></th>
        {/if}
      </tr>
    </thead>
    <tbody>
      {#each members as member, i (member.id)}
        <tr onclick={() => onselect?.(member)} class="trow" class:cursor-pointer={Boolean(onselect)}>
          <td class="td text-[11px] text-text/18 w-9">{pad2(i + 1)}</td>
          <td class="td text-[13px]">
            <div class="flex items-center gap-2.5">
              <div class="avatar w-7 h-7 text-[12px]">
                {initial(member.username)}
              </div>
              <span class="text-muted-text font-medium">{member.username}</span>
              {#if member.id === meId}
                <span class="text-[9px] tracking-[0.1em] text-accent bg-accent/12 border border-accent/28 py-px px-1.5">
                  you
                </span>
              {/if}
            </div>
          </td>
          <td class="td text-[13px] font-bold text-pts text-center">{member.score}</td>
          <td class="td text-[12px] text-text/45 text-center">{member.solves ?? "—"}</td>
          {#if teamScore !== null}
            {@const pct = teamScore > 0 ? Math.round((member.score / teamScore) * 100) : 0}
            <td class="td">
              <div class="flex items-center gap-2.5 min-w-35">
                <div class="flex-1 h-0.5 bg-background overflow-hidden">
                  <div class="h-full bg-accent" style="width: {pct}%"></div>
                </div>
                <span class="text-[10px] text-text/25 min-w-7 text-right">{pct}%</span>
              </div>
            </td>
          {/if}
          {#if hasActions}
            <td class="td text-right whitespace-nowrap">
              {#if member.id !== meId}
                {#if oncaptain}
                  <button
                    onclick={(e) => {
                      e.stopPropagation();
                      oncaptain(member);
                    }}
                    class="bg-transparent border-none text-[10px] tracking-[0.06em] text-text/30
                      cursor-pointer px-1 hover:text-accent"
                  >
                    Make captain
                  </button>
                {/if}
                {#if onkick}
                  <button
                    onclick={(e) => {
                      e.stopPropagation();
                      onkick(member);
                    }}
                    aria-label="Remove {member.username}"
                    class="bg-transparent border-none text-[11px] text-text/30 cursor-pointer px-1
                      hover:text-wrong"
                  >
                    <X size={13} />
                  </button>
                {/if}
              {/if}
            </td>
          {/if}
        </tr>
      {/each}
    </tbody>
  </table>
</div>
