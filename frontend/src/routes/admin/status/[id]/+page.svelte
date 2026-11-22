<script lang="ts">
  import { ArrowLeft } from "lucide-svelte";
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  let rows = $derived(
    data.instance
      ? [
          { label: "Image", value: data.instance.image },
          { label: "Status", value: data.instance.status },
          { label: "Provider", value: data.instance.provider },
          { label: "Created", value: new Date(data.instance.created_at).toLocaleString() },
          {
            label: "Host",
            value:
              data.instance.host_ip && data.instance.host_port
                ? `${data.instance.host_ip}:${data.instance.host_port}`
                : "—",
          },
        ]
      : [],
  );
</script>

<svelte:head>
  <title>Container: {data.instance?.name ?? "unknown"}</title>
</svelte:head>

<div class="min-h-page mt-nav bg-raised text-text">
  <div class="max-w-[48rem] mx-auto p-6 flex flex-col gap-4">
    <a href="/admin/status" class="back-btn self-start no-underline">
      <ArrowLeft size="0.875rem" />
      Status
    </a>

    <section class="card overflow-hidden">
      {#if data.instance}
        <div class="px-5 pt-4 pb-3">
          <div class="card-title">
            {data.instance.name}
          </div>
          <div class="font-mono text-label text-text/35 mt-0.5">
            {data.instance.id}
          </div>
        </div>
        {#each rows as row (row.label)}
          <div class="flex items-center gap-4 px-5 py-2.5 border-t border-text/5">
            <span class="w-24 shrink-0 text-label tracking-[0.1em] text-text/35">
              {row.label}
            </span>
            <span class="font-mono text-body text-muted-text break-all">
              {row.value}
            </span>
          </div>
        {/each}
      {:else}
        <div class="py-14 px-6 text-center">
          <div class="text-body text-text/60">No such container</div>
          <div class="text-label text-text/30 mt-1">
            {data.error ? "The backend has no instance API right now." : "It may have been stopped."}
          </div>
        </div>
      {/if}
    </section>
  </div>
</div>
