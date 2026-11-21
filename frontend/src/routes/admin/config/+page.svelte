<script lang="ts">
  import { invalidateAll } from "$app/navigation";
  import betaLogo from "@assets/beta.svg?raw";
  import logo from "@assets/logo.png";
  import * as api from "@library/api";
  import { THEMES } from "@library/themes";
  import type { ThemeId } from "@library/types";
  import { Check } from "lucide-svelte";
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  let saving = $state<ThemeId | null>(null);
  let error = $state("");
  let active = $derived(data.theme ?? "uiactf");

  async function pick(theme: ThemeId) {
    if (theme === active || saving) return;
    saving = theme;
    error = "";
    try {
      await api.updateSettings({ theme });
      await invalidateAll();
    } catch (e) {
      error = e instanceof Error ? e.message : "Could not save the theme";
    }
    saving = null;
  }
</script>

<svelte:head><title>Admin: Config</title></svelte:head>

<div class="min-h-[calc(100vh-3rem)] mt-12 bg-raised text-text flex justify-center">
  <div class="w-full max-w-[48rem] py-10 px-6 flex flex-col gap-6">
    <h1 class="text-2xl font-bold text-text/92 m-0">Config</h1>

    <section class="rounded-lg bg-elevated border border-text/6 p-5">
      <div class="text-[0.8125rem] font-medium text-text/80 mb-4">Theme</div>

      <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
        {#each THEMES as t (t.id)}
          {@const selected = t.id === active}
          <button
            onclick={() => pick(t.id)}
            aria-label={t.name}
            disabled={saving !== null}
            class="group relative text-left rounded-md border p-4 cursor-pointer transition-colors duration-100
              disabled:cursor-wait
              {selected ? 'border-accent bg-accent/8' : 'border-text/10 bg-background/40 hover:border-text/25'}"
          >
            <div class="h-6 flex items-center mb-3" style="color: {t.swatches[3]}">
              {#if t.id === "betauia"}
                <span class="block h-full [&>svg]:h-full [&>svg]:w-auto">{@html betaLogo}</span>
              {:else}
                <img src={logo} alt="" class="h-full w-auto block" />
              {/if}
            </div>
            <div class="flex items-center gap-1.5">
              {#each t.swatches as color (color)}
                <span class="w-6 h-6 rounded border border-text/10" style="background: {color}"></span>
              {/each}
            </div>
            {#if selected}
              <span
                class="absolute top-3 right-3 flex items-center justify-center w-5 h-5 rounded-full bg-accent
                  text-white"
              >
                <Check size="0.75rem" />
              </span>
            {:else if saving === t.id}
              <span class="absolute top-3 right-3 text-[0.6875rem] text-text/40">Saving...</span>
            {/if}
          </button>
        {/each}
      </div>

      {#if error}
        <div class="mt-3 text-[0.75rem] text-wrong">{error}</div>
      {/if}
    </section>
  </div>
</div>
