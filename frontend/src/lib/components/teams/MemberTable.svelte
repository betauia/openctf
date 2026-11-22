<script lang="ts">
  import type { Member } from "@library/types";
  import { initial } from "@library/utils";
  import { Crown, UserPlus, X } from "lucide-svelte";

  let {
    members,
    teamScore,
    meId,
    captainId,
    maxTeamSize = null,
    onselect,
    onkick,
    oncaptain,
  }: {
    members: Member[];
    teamScore: number;
    meId: string;
    captainId: string | null;
    maxTeamSize?: number | null;
    onselect: (member: Member) => void;
    onkick?: (member: Member) => void;
    oncaptain?: (member: Member) => void;
  } = $props();

  let openSlots = $derived(maxTeamSize ? Math.max(0, maxTeamSize - members.length) : 0);
</script>

<section class="card overflow-hidden">
  <div class="flex items-center justify-between px-5 pt-4 pb-3">
    <div class="card-title">Members</div>
    <div class="text-label text-text/30 tabular-nums">
      {members.length}{maxTeamSize ? ` / ${maxTeamSize}` : ""}
    </div>
  </div>

  {#each members as member (member.id)}
    {@const pct = teamScore ? Math.round((member.score / teamScore) * 100) : 0}
    {@const isMe = member.id === meId}
    <div
      role="button"
      tabindex="0"
      onclick={() => onselect(member)}
      onkeydown={(e) => e.key === "Enter" && onselect(member)}
      class="group flex items-center gap-4 px-5 py-3 border-t border-text/5 cursor-pointer
        transition-colors duration-100 hover:bg-text/3"
    >
      <div class="avatar w-9 h-9 text-body rounded-md">
        {initial(member.username)}
      </div>

      <div class="flex-1 min-w-0">
        <div class="flex items-center gap-2">
          <span class="text-body font-medium text-muted-text truncate">
            {member.username}
          </span>
          {#if member.id === captainId}
            <span
              title="Captain"
              class="inline-flex items-center gap-1 text-3xs tracking-[0.1em] text-pts bg-pts/10
                border border-pts/25 py-px px-1.5"
            >
              <Crown size="0.5625rem" />
              captain
            </span>
          {/if}
          {#if isMe}
            <span class="text-3xs tracking-[0.1em] text-accent bg-accent/12 border border-accent/28 py-px px-1.5">
              you
            </span>
          {/if}
        </div>
        <div class="text-label text-text/30 mt-0.5">
          {member.solves}
          {member.solves === 1 ? "solve" : "solves"}
        </div>
      </div>

      <div class="hidden sm:flex items-center gap-2.5 w-40">
        <div class="flex-1 h-1 rounded-full bg-background overflow-hidden">
          <div class="h-full rounded-full bg-accent" style="width: {pct}%"></div>
        </div>
        <span class="text-2xs text-text/30 w-8 text-right tabular-nums">
          {pct}%
        </span>
      </div>

      <div class="w-14 text-right text-body font-bold text-pts tabular-nums">
        {member.score}
      </div>

      {#if (onkick || oncaptain) && members.length > 1}
        <div class="flex items-center gap-1 w-24 justify-end">
          {#if !isMe}
            {#if oncaptain}
              <button
                onclick={(e) => {
                  e.stopPropagation();
                  if (confirm(`Make ${member.username} captain? You will lose captain rights.`)) oncaptain(member);
                }}
                title="Make captain"
                aria-label="Make {member.username} captain"
                class="opacity-0 group-hover:opacity-100 focus:opacity-100 flex items-center justify-center w-7 h-7
                  rounded bg-transparent border-none text-text/40 cursor-pointer transition-all duration-100
                  hover:text-pts hover:bg-pts/10"
              >
                <Crown size="0.8125rem" />
              </button>
            {/if}
            {#if onkick}
              <button
                onclick={(e) => {
                  e.stopPropagation();
                  if (confirm(`Remove ${member.username} from the team?`)) onkick(member);
                }}
                title="Remove from team"
                aria-label="Remove {member.username}"
                class="opacity-0 group-hover:opacity-100 focus:opacity-100 flex items-center justify-center w-7 h-7
                  rounded bg-transparent border-none text-text/40 cursor-pointer transition-all duration-100
                  hover:text-wrong hover:bg-wrong/10"
              >
                <X size="0.875rem" />
              </button>
            {/if}
          {/if}
        </div>
      {/if}
    </div>
  {/each}

  {#each { length: openSlots } as _, i (i)}
    <div class="flex items-center gap-4 px-5 py-3 border-t border-text/5">
      <div
        class="w-9 h-9 rounded-md border border-dashed border-text/15 flex items-center justify-center
          text-text/20 shrink-0"
      >
        <UserPlus size="0.875rem" />
      </div>
      <span class="text-xs text-text/25">Open slot</span>
    </div>
  {/each}
</section>
