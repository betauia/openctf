<script lang="ts">
  import { Check, Copy } from "lucide-svelte";

  type Props = { text: string; size?: number; title?: string; class?: string };
  let { text, size = 14, title = "Copy", class: className = "" }: Props = $props();

  let copied = $state(false);
  let timer: ReturnType<typeof setTimeout>;

  function copy() {
    navigator.clipboard.writeText(text);
    copied = true;
    clearTimeout(timer);
    timer = setTimeout(() => (copied = false), 1500);
  }
</script>

<button {title} onclick={copy} class={className}>
  {#if copied}
    <Check {size} class="text-correct animate-[icon-pop_0.15s_ease-out]" />
  {:else}
    <Copy {size} />
  {/if}
</button>
