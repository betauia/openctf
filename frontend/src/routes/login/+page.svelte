<script lang="ts">
  import { goto } from "$app/navigation";
  import { page } from "$app/state";
  import * as api from "@library/api";
  import { afterLogin, SIGN_IN_REASONS } from "@library/redirect";

  let username = $state("");
  let password = $state("");
  let error = $state("");
  let busy = $state(false);

  let reason = $derived(SIGN_IN_REASONS[page.url.searchParams.get("reason") ?? ""]);

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    busy = true;
    error = "";
    try {
      await api.login({ username, password });
      // invalidateAll so +layout.ts re-runs and the nav picks up the new user.
      await goto(afterLogin(page.url), { invalidateAll: true });
    } catch (e) {
      error = e instanceof Error ? e.message : "Login failed";
      busy = false;
    }
  }
</script>

<svelte:head><title>Login — OpenCTF</title></svelte:head>

<div class="bg-raised text-text pt-12 min-h-screen flex flex-col items-center">
  <div class="max-w-[75rem] w-full px-12 flex-1 flex flex-col">
    <div class="flex-1 mx-[-3rem] flex items-center justify-center">
      <div class="bg-elevated border border-text/4 py-8 px-9 w-[clamp(300px,28vw,420px)]">
        <h1 class="text-2xl font-bold text-text/92 m-0 mb-2">Sign in</h1>

        {#if reason}
          <p class="text-[12px] leading-[1.6] text-wrong/80 mt-0 mb-4">{reason}</p>
        {:else}
          <div class="mb-3"></div>
        {/if}

        <form onsubmit={submit}>
          <div class="flex flex-col gap-1.5 mb-4">
            <label for="username" class="text-[11px] text-text/45 tracking-[0.08em]">Username</label>
            <input id="username" type="text" autocomplete="username" required bind:value={username} class="field" />
          </div>

          <div class="flex flex-col gap-1.5 mb-4">
            <label for="password" class="text-[11px] text-text/45 tracking-[0.08em]">Password</label>
            <input
              id="password"
              type="password"
              autocomplete="current-password"
              required
              bind:value={password}
              class="field"
            />
          </div>

          <div class="text-[11px] text-accent min-h-0 mb-1.5">{error}</div>

          <button type="submit" disabled={busy} class="btn-primary">
            {busy ? "..." : "Sign in"}
          </button>
        </form>

        <p class="text-[12px] text-text/35 mt-5 mx-0 mb-0 text-center">
          No account? <a href="/register" class="text-accent no-underline">Register</a>
        </p>
      </div>
    </div>
  </div>
</div>
