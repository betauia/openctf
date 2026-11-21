<script lang="ts">
  import { Search } from "lucide-svelte";
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  let filter = $state("");

  let shown = $derived(
    data.instances.filter((i) =>
      [i.id, i.name, i.image, i.status, new Date(i.created_at).toLocaleString()]
        .join(" ")
        .toUpperCase()
        .includes(filter.toUpperCase()),
    ),
  );
</script>

<svelte:head><title>Admin: Status</title></svelte:head>

<div class="bg-raised text-text pt-12 min-h-screen flex flex-col items-center">
  <div class="max-w-[75rem] w-full px-4 sm:px-12 flex-1 flex flex-col">
    <div class="flex flex-col items-center gap-4 overflow-y-auto max-h-[40rem] max-w-[calc(100%-1rem)] mx-auto px-6">
      <div class="mt-4 flex items-center justify-between gap-2.5 -mb-2.5 w-full">
        <div class="flex items-center gap-1.5">
          <span class="text-[1.125rem] text-accent">▣</span>
          <h1 class="m-0 text-[1.125rem] font-bold tracking-[-0.02em] text-text">challenge containers</h1>
        </div>
        <div class="relative w-2/5 max-w-80">
          <Search
            size="0.875rem"
            class="absolute left-2.5 top-1/2 -translate-y-1/2 text-muted-text pointer-events-none"
          />
          <input
            type="text"
            placeholder="Filter searches..."
            bind:value={filter}
            class="w-full bg-raised text-text border-[0.5px] border-raised rounded-[0.3125rem] pl-8 box-border"
          />
        </div>
      </div>

      <div class="w-full overflow-x-auto">
        <table
          class="table-fixed w-full min-w-[40rem] text-center text-sm text-text border-separate border-[0.5px] border-raised
          rounded-[0.625rem] border-spacing-0 overflow-hidden"
        >
          <thead>
            <tr>
              {#each ["ID", "NAME", "IMAGE", "STATUS", "CREATED"] as header (header)}
                <th class="p-2 min-w-28 max-w-80 bg-raised text-muted-text">{header}</th>
              {/each}
            </tr>
          </thead>
          <tbody>
            {#if shown.length === 0}
              <tr><td colspan="5" class="p-2 min-w-28 max-w-80">No instances found.</td></tr>
            {:else}
              {#each shown as instance (instance.id)}
                <tr>
                  <td class="p-2 min-w-28 max-w-80">
                    <a href="/admin/status/{instance.id}" class="no-underline hover:underline">
                      {instance.id}
                    </a>
                  </td>
                  <td class="p-2 min-w-28 max-w-80">{instance.name}</td>
                  <td class="p-2 min-w-28 max-w-80">{instance.image}</td>
                  <td class="p-2 min-w-28 max-w-80">{instance.status}</td>
                  <td class="p-2 min-w-28 max-w-80">{new Date(instance.created_at).toLocaleString()}</td>
                </tr>
              {/each}
            {/if}
          </tbody>
        </table>
      </div>
    </div>
  </div>
</div>
