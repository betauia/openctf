<script lang="ts">
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();
</script>

<svelte:head><title>Rules — {data.settings.ctf_name}</title></svelte:head>

<div class="bg-raised text-text pt-12 min-h-screen flex flex-col items-center">
  <div class="max-w-[75rem] w-full px-12 flex-1 flex flex-col">
    <div class="py-10 max-w-[48rem]">
      <h1 class="text-2xl font-bold text-text/92 m-0 mb-8">Rules</h1>

      {#if data.settings.rules}
        <div class="whitespace-pre-wrap text-[13px] leading-[1.75] text-text/70">
          {data.settings.rules}
        </div>
      {:else}
        <p class="text-[13px] leading-[1.75] text-text/35 m-0">
          No rules have been published yet. An organiser can write them in the admin panel.
        </p>
      {/if}
    </div>
  </div>
</div>
