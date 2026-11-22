<script lang="ts">
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();
</script>

<svelte:head><title>Rules — {data.settings.ctf_name}</title></svelte:head>

<div class="min-h-page mt-nav bg-raised text-text">
  <div class="max-w-[48rem] mx-auto p-6 flex flex-col gap-4">
    <h1 class="m-0 mt-4 text-2xl font-bold text-text">Rules</h1>
    <section class="card p-6">
      {#if data.settings.rules}
        <div class="whitespace-pre-wrap text-sm leading-[1.8] text-text/75">{data.settings.rules}</div>
      {:else}
        <p class="m-0 py-6 text-center text-body text-text/35">No rules have been published yet.</p>
      {/if}
    </section>
  </div>
</div>
