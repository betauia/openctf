<script lang="ts">
  import { ArrowRight, Check, Flag } from "lucide-svelte";

  export type FlagResult = { ok: boolean; message: string };

  let {
    label,
    placeholder,
    large = false,
    onsubmit,
  }: {
    label: string;
    placeholder: string;
    large?: boolean;
    onsubmit: (flag: string) => Promise<FlagResult>;
  } = $props();

  let value = $state("");
  let result = $state<FlagResult | null>(null);

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    const flag = value.trim();
    if (!flag) return;
    result = null;
    result = await onsubmit(flag);
    if (result.ok) value = "";
  }

  let buttonColor = $derived(
    result?.ok
      ? "bg-correct-dark text-white"
      : result
        ? "bg-wrong-dark text-white"
        : value.trim()
          ? "bg-accent text-white hover:bg-accent-dark"
          : "bg-text/6 text-text/30",
  );
</script>

<div class={large ? "w-full" : "flex-1 min-w-0 sm:flex-none sm:w-80"}>
  <form
    onsubmit={submit}
    data-form-type="other"
    class="flex items-center gap-2 m-0 rounded-md bg-background border border-text/10 transition-colors duration-100
      focus-within:border-accent/60 {large ? 'h-[2.5rem] pl-3.5 pr-1.5' : 'h-[2.125rem] pl-3 pr-1'}"
  >
    <Flag size={large ? "0.875rem" : "0.8125rem"} class="shrink-0 text-text/30" />
    <input
      type="text"
      {placeholder}
      aria-label={label}
      autocomplete="off"
      data-lpignore="true"
      bind:value
      oninput={() => (result = null)}
      class="flex-1 min-w-0 bg-transparent border-none outline-none p-0 font-mono text-muted-text
        {large ? 'text-[0.8125rem]' : 'text-[0.75rem]'}"
    />
    <button
      type="submit"
      aria-label="Submit flag"
      disabled={!value.trim()}
      class="flex items-center justify-center shrink-0 rounded border-none cursor-pointer transition-colors
        duration-150 disabled:cursor-default {buttonColor}
        {large ? 'w-[1.875rem] h-[1.875rem]' : 'w-[1.625rem] h-[1.625rem]'}"
    >
      {#if result?.ok}
        <Check size={large ? "0.875rem" : "0.8125rem"} />
      {:else}
        <ArrowRight size={large ? "0.875rem" : "0.8125rem"} />
      {/if}
    </button>
  </form>
  {#if result?.message}
    <div class="mt-1.5 text-[0.6875rem]" class:text-correct={result.ok} class:text-wrong={!result.ok}>
      {result.message}
    </div>
  {/if}
</div>
