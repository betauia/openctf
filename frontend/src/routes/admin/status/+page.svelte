<script lang="ts">
  import { Box, Search } from "lucide-svelte";
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  let filter = $state("");

  let shown = $derived(
    data.instances.value.filter((i) =>
      [i.id, i.name, i.image, i.status].join(" ").toLowerCase().includes(filter.trim().toLowerCase()),
    ),
  );
</script>

<svelte:head><title>Admin: Status</title></svelte:head>

<div class="min-h-page mt-nav bg-raised text-text">
  <div class="max-w-[75rem] mx-auto p-6 flex flex-col gap-4">
    <section class="card overflow-hidden">
      <div class="flex flex-wrap items-center justify-between gap-3 px-5 pt-4 pb-3">
        <div>
          <div class="card-title">Challenge containers</div>
          <div class="text-label text-text/35 mt-0.5">
            {data.instances.value.length} running
          </div>
        </div>
        <label
          class="flex items-center gap-2 w-full sm:w-64 h-[2.125rem] px-3 rounded-md bg-background border
            border-text/10 transition-colors duration-100 focus-within:border-accent/60 cursor-text"
        >
          <Search size="0.8125rem" class="shrink-0 text-text/30" />
          <input
            type="text"
            placeholder="Filter containers..."
            bind:value={filter}
            class="flex-1 min-w-0 bg-transparent border-none outline-none p-0 text-xs text-muted-text"
          />
        </label>
      </div>

      {#if data.instances.error}
        <div class="flex flex-col items-center gap-2 py-14 px-6 border-t border-text/5 text-center">
          <Box size="1.5rem" class="text-text/20" />
          <div class="text-body text-text/60">Container instances aren't available yet</div>
          <div class="text-label text-text/30 max-w-[28rem]">
            The backend has no instance API right now, so there's nothing to list.
          </div>
        </div>
      {:else if shown.length === 0}
        <div class="py-14 text-center text-xs text-text/30 border-t border-text/5">
          {data.instances.value.length === 0 ? "No containers are running." : `Nothing matches "${filter}".`}
        </div>
      {:else}
        <div class="overflow-x-auto">
          <table class="w-full min-w-[40rem] border-collapse text-left">
            <thead>
              <tr>
                {#each ["ID", "Name", "Image", "Status", "Created"] as header (header)}
                  <th class="th border-t border-text/5">{header}</th>
                {/each}
              </tr>
            </thead>
            <tbody>
              {#each shown as instance (instance.id)}
                <tr class="border-t border-text/5 transition-colors duration-100 hover:bg-text/3">
                  <td class="td font-mono text-xs">
                    <a href="/admin/status/{instance.id}" class="text-accent no-underline hover:underline">
                      {instance.id.slice(0, 12)}
                    </a>
                  </td>
                  <td class="td text-body text-muted-text">{instance.name}</td>
                  <td class="td font-mono text-xs text-text/60">
                    {instance.image}
                  </td>
                  <td class="td text-xs text-text/60">{instance.status}</td>
                  <td class="td text-xs text-text/40">
                    {new Date(instance.created_at).toLocaleString()}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </section>
  </div>
</div>
