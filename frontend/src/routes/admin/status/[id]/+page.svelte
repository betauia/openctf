<script lang="ts">
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();
</script>

<svelte:head><title>Container: {data.instance?.id ?? "unknown"}</title></svelte:head>

<div class="bg-raised text-text pt-12 min-h-screen flex flex-col items-center">
  <div class="max-w-[75rem] w-full px-12 flex-1 flex flex-col">
    <div class="flex flex-col items-center gap-4 mt-4 border border-raised rounded-[20px] p-4 text-text">
      {#if data.instance}
        <div>
          <h3>Basic Info</h3>
          <p><strong>ID:</strong> {data.instance.id}</p>
          <p><strong>Name:</strong> {data.instance.name}</p>
          <p><strong>Image:</strong> {data.instance.image}</p>
          <p><strong>Status:</strong> {data.instance.status}</p>
          <p><strong>Created:</strong> {new Date(data.instance.created_at).toLocaleString()}</p>
          <p><strong>Provider:</strong> {data.instance.provider}</p>
        </div>
      {:else}
        <p>No such instance.</p>
      {/if}
    </div>
  </div>
</div>
