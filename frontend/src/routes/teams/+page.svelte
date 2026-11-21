<script lang="ts">
  import { goto, invalidateAll } from "$app/navigation";
  import LeaderboardTab from "@components/teams/LeaderboardTab.svelte";
  import MemberDetail from "@components/teams/MemberDetail.svelte";
  import TeamTab from "@components/teams/TeamTab.svelte";
  import * as api from "@library/api";
  import type { ActivityEntry, Member, Team } from "@library/types";
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  let teams = $state(data.teams);
  let myTeam = $state(data.myTeam);
  let activity = $state(data.activity.value);

  $effect(() => {
    teams = data.teams;
    myTeam = data.myTeam;
    activity = data.activity.value;
  });

  let tab = $state<"team" | "leaderboard">("team");
  let inspecting = $state<Team | null>(null);
  let inspectingActivity = $state<ActivityEntry[]>([]);
  let inspectingActivityError = $state<string | null>(null);
  let error = $state("");

  let member = $derived(data.member.value);

  async function inspectTeam(team: Team) {
    inspecting = team;
    inspectingActivity = [];
    inspectingActivityError = null;
    try {
      inspectingActivity = await api.teamActivity(team.id);
    } catch (e) {
      inspectingActivityError = e instanceof Error ? e.message : "Could not load activity";
    }
  }

  async function act(action: () => Promise<unknown>) {
    error = "";
    try {
      await action();
      await invalidateAll();
    } catch (e) {
      error = e instanceof Error ? e.message : "Failed";
    }
  }
</script>

<svelte:head><title>Teams - OpenCTF</title></svelte:head>

<div class="flex flex-col h-[calc(100vh-48px)] mt-12 bg-raised">
  <div class="flex items-center justify-center gap-1 bg-elevated border-b border-text/6 shrink-0 h-10">
    {#each [{ id: "team", label: "My Team" }, { id: "leaderboard", label: "Leaderboard" }] as const as t (t.id)}
      <button
        onclick={() => {
          tab = t.id;
          inspecting = null;
          goto("/teams");
        }}
        class="flex items-center px-3.5 py-1 rounded-full border-none text-[11px]
          tracking-[0.1em] cursor-pointer transition-colors duration-100
          {tab === t.id ? 'text-text bg-accent/15' : 'text-text/35 bg-transparent hover:text-text/60'}"
      >
        {t.label}
      </button>
    {/each}
  </div>

  {#if data.member.error}
    <div class="flex-1 overflow-y-auto flex flex-col">
      <div class="empty text-wrong/70">{data.member.error}</div>
      <button onclick={() => goto("/teams")} class="back-btn mx-auto">Back</button>
    </div>
  {:else if member}
    <div class="flex-1 overflow-y-auto flex flex-col">
      <MemberDetail {member} backLabel="Back" onback={() => goto("/teams")} />
    </div>
  {:else if tab === "team"}
    <TeamTab
      {myTeam}
      me={data.user}
      {activity}
      activityError={data.activity.error}
      {error}
      onleave={() => act(api.leaveTeam)}
      oncreate={(name) => act(() => api.createTeam(name))}
      onjoin={(code) => act(() => api.joinTeam(code))}
      onmember={(m: Member) => goto(`/teams?member=${m.id}`)}
      onrename={(name) => act(() => api.updateTeam({ name }))}
      onregenerate={() => act(() => api.updateTeam({ regenerate_invite: true }))}
      onkick={(m: Member) => act(() => api.kickMember(m.id))}
      oncaptain={(m: Member) => act(() => api.transferCaptain(m.id))}
    />
  {:else}
    <LeaderboardTab
      {teams}
      {myTeam}
      {inspecting}
      activity={inspectingActivity}
      activityError={inspectingActivityError}
      onmine={() => (tab = "team")}
      oninspect={(team) => inspectTeam(team)}
      onback={() => (inspecting = null)}
      onmember={(m: Member) => goto(`/teams?member=${m.id}`)}
    />
  {/if}
</div>
