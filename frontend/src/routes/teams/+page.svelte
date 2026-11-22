<script lang="ts">
  import { goto, invalidateAll } from "$app/navigation";
  import MemberDetail from "#components/teams/MemberDetail.svelte";
  import TeamTab from "#components/teams/TeamTab.svelte";
  import * as api from "#library/api.js";
  import type { Member } from "#library/types.js";
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  let error = $state("");

  async function act(action: () => Promise<unknown>) {
    error = "";
    try {
      await action();
      await invalidateAll();
      return true;
    } catch (e) {
      error = e instanceof Error ? e.message : "Failed";
      return false;
    }
  }
</script>

<svelte:head><title>Teams - OpenCTF</title></svelte:head>

<div class="flex flex-col h-page mt-nav bg-raised text-muted-text">
  {#if data.member.error}
    <div class="flex-1 overflow-y-auto flex flex-col">
      <div class="empty text-wrong/70">{data.member.error}</div>
      <button onclick={() => goto("/teams")} class="back-btn mx-auto"> Back </button>
    </div>
  {:else if data.member.value}
    <div class="flex-1 overflow-y-auto">
      <div class="max-w-[75rem] mx-auto flex flex-col">
        <MemberDetail member={data.member.value} />
      </div>
    </div>
  {:else}
    <TeamTab
      myTeam={data.myTeam}
      me={data.user}
      activity={data.activity.value}
      activityError={data.activity.error}
      {error}
      maxTeamSize={data.maxTeamSize}
      progress={data.progress}
      oncreate={(name) => act(() => api.createTeam(name))}
      onjoin={(code) => act(() => api.joinTeam(code))}
      onmember={(m: Member) => goto(`/teams?member=${m.id}`)}
      onkick={(m: Member) => act(() => api.kickMember(m.id))}
      oncaptain={(m: Member) => act(() => api.transferCaptain(m.id))}
      onrename={(name) => act(() => api.updateTeam({ name }))}
      onregenerate={() => act(() => api.updateTeam({ regenerate_invite: true }))}
      onleave={() => act(api.leaveTeam)}
    />
  {/if}
</div>
