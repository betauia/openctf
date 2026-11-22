<script lang="ts">
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();
</script>

<svelte:head><title>Rules — {data.settings.ctf_name}</title></svelte:head>

<div class="min-h-[calc(100vh-3rem)] mt-12 bg-raised text-text">
  <div class="max-w-[48rem] mx-auto p-6 flex flex-col gap-4">
    <h1 class="m-0 mt-4 text-2xl font-bold text-text">Rules</h1>
    <section class="rounded-lg bg-elevated border border-text/6 p-6">
      {#if data.settings.rules}
        <div class="whitespace-pre-wrap text-[0.875rem] leading-[1.8] text-text/75">{data.settings.rules}</div>
      {:else}
        <p class="m-0 py-6 text-center text-[0.8125rem] text-text/35">No rules have been published yet.</p>
      {/if}
    </section>
  </div>
</div>
