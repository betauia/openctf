<script lang="ts">
  import { goto, invalidateAll } from "$app/navigation";
  import { page } from "$app/state";
  import betaLogo from "@assets/beta.svg?raw";
  import logo from "@assets/logo.png";
  import * as api from "@library/api";
  import { applyTheme } from "@library/themes";
  import type { SolveEvent } from "@library/types";
  import { initial } from "@library/utils";
  import { fly } from "svelte/transition";
  import { LogOut } from "lucide-svelte";
  import "@styles/global.css";

  let { children, data } = $props();
  let user = $derived(data.user);
  let theme = $derived(data.settings?.theme ?? "uiactf");

  $effect(() => applyTheme(theme));

  let userId = $derived(user?.id);
  let toasts = $state<(SolveEvent & { key: number; mine: boolean })[]>([]);
  let refreshTimer: ReturnType<typeof setTimeout> | undefined;
  let staleWhileHidden = false;

  function refresh() {
    if (document.hidden) {
      staleWhileHidden = true;
      return;
    }
    clearTimeout(refreshTimer);
    refreshTimer = setTimeout(() => invalidateAll(), 300);
  }

  let nextToast = 0;

  function showToast(solve: SolveEvent, me: string) {
    const key = nextToast++;
    toasts = [...toasts.slice(-3), { ...solve, key, mine: solve.user_id === me }];
    setTimeout(() => (toasts = toasts.filter((t) => t.key !== key)), 6000);
  }

  $effect(() => {
    const me = userId;
    if (!me) return;
    const controller = new AbortController();
    api.listen((event, data) => {
      if (event !== "solve") return;
      showToast(data as SolveEvent, me);
      refresh();
    }, controller.signal);
    return () => controller.abort();
  });

  let isAdmin = $derived(user?.role === "admin");

  let mainLinks = $derived([
    { name: "Challenges", href: "/challenges" },
    { name: "Teams", href: "/teams" },
    { name: "Scoreboard", href: "/scoreboard" },
    { name: "Rules", href: "/rules" },
  ]);

  let onAdmin = $derived(page.url.pathname.startsWith("/admin"));

  let adminLinks = $derived(
    isAdmin
      ? onAdmin
        ? [
            { name: "Status", href: "/admin/status" },
            { name: "Config", href: "/admin/config" },
          ]
        : [{ name: "Admin", href: "/admin/status" }]
      : [],
  );

  async function logout() {
    await api.logout();
    await goto("/", { invalidateAll: true });
  }
</script>

<svelte:document
  onvisibilitychange={() => {
    if (!document.hidden && staleWhileHidden) {
      staleWhileHidden = false;
      invalidateAll();
    }
  }}
/>

<nav class="fixed top-0 left-0 w-full h-12 bg-background border-b border-text/7 z-50">
  <div class="h-full flex items-stretch">
    <a href="/" class="flex items-center pl-4 pr-3 sm:pl-5 sm:pr-6 shrink-0">
      {#if theme === "betauia"}
        <span class="block h-5 text-accent [&>svg]:h-full [&>svg]:w-auto">{@html betaLogo}</span>
      {:else}
        <img src={logo} alt="OpenCTF" width="242" height="56" class="h-5 w-auto block opacity-90" />
      {/if}
    </a>

    <div class="flex items-center gap-1 flex-1 min-w-0 overflow-x-auto [scrollbar-width:none]">
      {#each mainLinks as link (link.href)}
        {@const active = page.url.pathname.startsWith(link.href)}
        <a
          href={link.href}
          class="flex items-center shrink-0 px-3 py-1 rounded-full no-underline text-[0.6875rem] tracking-[0.1em]
            transition-colors duration-100
            {active ? 'text-text bg-accent/15' : 'text-text/35 hover:text-text/60'}"
        >
          {link.name}
        </a>
      {/each}
      {#if adminLinks.length > 0}
        <div class="w-px h-4 shrink-0 bg-text/10 mx-1.5"></div>
        {#each adminLinks as link (link.href)}
          {@const active = page.url.pathname.startsWith(link.href)}
          <a
            href={link.href}
            class="flex items-center shrink-0 px-3 py-1 rounded-full no-underline text-[0.6875rem] tracking-[0.1em]
              transition-colors duration-100
              {active ? 'text-text bg-accent/15' : 'text-text/35 hover:text-text/60'}"
          >
            {link.name}
          </a>
        {/each}
      {/if}
    </div>

    <div class="flex items-center gap-3 pl-2 pr-3 sm:pr-5 shrink-0">
      {#if user}
        <div class="flex items-center h-8 border border-text/8 rounded-md bg-text/3">
          <div class="flex items-center gap-2 pl-1 pr-1 sm:pr-3">
            {#if user.avatar_url}
              <img src={user.avatar_url} alt="" class="w-6 h-6 rounded object-cover shrink-0" />
            {:else}
              <div class="avatar w-6 h-6 text-[0.625rem] rounded">
                {user.username[0].toUpperCase()}
              </div>
            {/if}
            <span class="hidden sm:inline text-[0.75rem] text-muted-text">{user.username}</span>
          </div>
          <button
            onclick={logout}
            title="Logout"
            aria-label="Logout"
            class="flex items-center justify-center h-full px-2.5 border-l border-text/8 bg-transparent
              text-text/35 cursor-pointer rounded-r-md transition-colors duration-100
              hover:text-accent hover:bg-accent/10"
          >
            <LogOut size="0.8125rem" />
          </button>
        </div>
      {:else}
        <a
          href="/login"
          class="inline-flex items-center h-7 text-[0.6875rem] tracking-[0.08em] text-accent
            bg-transparent border border-accent/30 rounded-sm px-3.5 cursor-pointer no-underline
            transition-colors hover:bg-accent/10 hover:border-accent/50"
        >
          Login
        </a>
      {/if}
    </div>
  </div>
</nav>

{@render children()}

<div class="fixed bottom-4 right-4 z-[60] flex flex-col gap-2 w-[20rem] max-w-[calc(100vw-2rem)] pointer-events-none">
  {#each toasts as toast (toast.key)}
    <a
      href="/challenges?open={toast.challenge_id}"
      in:fly={{ x: 24, duration: 200 }}
      out:fly={{ x: 24, duration: 200 }}
      class="pointer-events-auto flex items-center gap-3 rounded-lg bg-elevated border py-2.5 px-3 no-underline
        {toast.mine ? 'border-accent/50' : 'border-text/10'}"
    >
      <div class="avatar w-8 h-8 text-[0.75rem] rounded-md">{initial(toast.username)}</div>
      <div class="flex-1 min-w-0">
        <div class="text-[0.75rem] text-muted-text truncate">
          <span class="font-semibold text-text">{toast.mine ? "You" : toast.username}</span>
          solved
          <span class="cat-{toast.category} px-1">{toast.challenge}</span>
        </div>
        {#if toast.team}
          <div class="text-[0.6875rem] text-text/40 truncate">{toast.team}</div>
        {/if}
      </div>
      <span class="text-[0.75rem] font-bold text-pts tabular-nums shrink-0">+{toast.points}</span>
    </a>
  {/each}
</div>
