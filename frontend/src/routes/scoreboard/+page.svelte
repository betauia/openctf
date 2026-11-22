<script lang="ts">
  import LeaderboardTab from "#components/teams/LeaderboardTab.svelte";
  import ScoreChart from "#components/scoreboard/ScoreChart.svelte";
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();
</script>

<svelte:head><title>Scoreboard - OpenCTF</title></svelte:head>

<div class="flex flex-col h-page mt-nav bg-raised overflow-y-auto">
  <div class="w-full max-w-[75rem] mx-auto">
    {#if data.teams.length === 0}
      <div class="flex-1 flex items-center justify-center text-body text-text/30">No teams yet</div>
    {:else}
      {#if data.series.length > 0}
        <ScoreChart series={data.series} start={data.start} end={data.end} />
      {/if}
      <LeaderboardTab teams={data.teams} myTeam={data.myTeam} />
    {/if}
  </div>
</div>
