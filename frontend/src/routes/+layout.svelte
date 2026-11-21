<script lang="ts">
  import { goto } from "$app/navigation";
  import { page } from "$app/state";
  import logo from "@assets/logo.png";
  import * as api from "@library/api";
  import "@styles/global.css";

  let { children, data } = $props();
  let user = $derived(data.user);

  let isAdmin = $derived(user?.role === "admin");

  let onAdmin = $derived(page.url.pathname.startsWith("/admin"));

  let mainLinks = $derived([
    { name: "Challenges", href: "/challenges" },
    { name: "Teams", href: "/teams" },
    { name: "Scoreboard", href: "/scoreboard" },
    { name: "Rules", href: "/rules" },
  ]);

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

<nav class="fixed top-0 left-0 w-full h-12 bg-background border-b border-text/7 z-50">
  <div class="h-full flex items-stretch max-w-[1440px] mx-auto">
    <a href="/" class="flex items-center pl-5 pr-6 shrink-0">
      <img src={logo} alt="OpenCTF" width="242" height="56" class="h-5 w-auto block opacity-90" />
    </a>

    <div class="flex items-center gap-1 flex-1">
      {#each mainLinks as link (link.href)}
        {@const active = page.url.pathname.startsWith(link.href)}
        <a
          href={link.href}
          class="flex items-center px-3 py-1 rounded-full no-underline text-[11px] tracking-[0.1em]
            transition-colors duration-100
            {active ? 'text-text bg-accent/15' : 'text-text/35 hover:text-text/60'}"
        >
          {link.name}
        </a>
      {/each}
      {#if adminLinks.length > 0}
        <div class="w-px h-4 bg-text/10 mx-1.5"></div>
        {#each adminLinks as link (link.href)}
          {@const active = page.url.pathname.startsWith(link.href)}
          <a
            href={link.href}
            class="flex items-center px-3 py-1 rounded-full no-underline text-[11px] tracking-[0.1em]
              transition-colors duration-100
              {active ? 'text-text bg-accent/15' : 'text-text/35 hover:text-text/60'}"
          >
            {link.name}
          </a>
        {/each}
      {/if}
    </div>

    <div class="flex items-center gap-3 pr-5 shrink-0">
      {#if user}
        <span class="text-[11px] text-muted-text tracking-[0.04em]">{user.username}</span>
        <div class="avatar w-6 h-6 text-[10px] rounded-sm">
          {user.username[0].toUpperCase()}
        </div>
        <button
          onclick={logout}
          class="text-[10px] tracking-[0.08em] text-accent bg-transparent border border-accent/30
            rounded-sm px-2.5 py-1 cursor-pointer transition-colors duration-100
            hover:bg-accent/10 hover:border-accent/50"
        >
          Logout
        </button>
      {:else}
        <a
          href="/login"
          class="inline-flex items-center h-7 text-[11px] tracking-[0.08em] text-accent
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
