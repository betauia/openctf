<script lang="ts">
  import { page } from "$app/state";
  import { ArrowLeft, House } from "lucide-svelte";

  let notFound = $derived(page.status === 404);
</script>

<svelte:head><title>{page.status} - OpenCTF</title></svelte:head>

<div class="min-h-page mt-nav bg-raised text-text flex items-center justify-center px-6">
  <div class="w-full max-w-[28rem] card rounded-xl p-8 text-center">
    <div class="text-5xl font-bold text-accent tabular-nums">{page.status}</div>
    <h1 class="m-0 mt-3 text-lg font-bold text-text">
      {notFound ? "Page not found" : "Something went wrong"}
    </h1>
    <p class="m-0 mt-2 text-body leading-[1.7] text-text/45">
      {notFound ? "There's nothing at this address." : (page.error?.message ?? "An unexpected error occurred.")}
    </p>
    <div class="flex items-center justify-center gap-2 mt-6">
      <button
        onclick={() => history.back()}
        class="inline-flex items-center gap-1.5 rounded-md border border-text/10 bg-transparent py-2 px-3.5
          text-xs text-text/60 cursor-pointer transition-colors duration-100 hover:text-text
          hover:border-text/25"
      >
        <ArrowLeft size="0.8125rem" />
        Back
      </button>
      <a
        href="/challenges"
        class="inline-flex items-center gap-1.5 rounded-md bg-accent py-2 px-3.5 text-xs font-semibold
          text-white no-underline transition-colors duration-100 hover:bg-accent-dark"
      >
        <House size="0.8125rem" />
        Challenges
      </a>
    </div>
  </div>
</div>
