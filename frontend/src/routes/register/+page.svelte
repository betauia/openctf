<script lang="ts">
  import { goto } from "$app/navigation";
  import { page } from "$app/state";
  import * as api from "@library/api";
  import { afterLogin } from "@library/redirect";

  let username = $state("");
  let email = $state("");
  let password = $state("");
  let error = $state("");
  let busy = $state(false);

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    busy = true;
    error = "";
    try {
      await api.register({ username, email, password });
      // invalidateAll so +layout.ts re-runs and the nav picks up the new user.
      await goto(afterLogin(page.url), { invalidateAll: true });
    } catch (e) {
      error = e instanceof Error ? e.message : "Registration failed";
      busy = false;
    }
  }
</script>

<svelte:head><title>Register — OpenCTF</title></svelte:head>

<div class="bg-raised text-text pt-12 min-h-screen flex flex-col items-center">
  <div class="max-w-[75rem] w-full px-12 flex-1 flex flex-col">
    <div class="flex-1 mx-[-3rem] flex items-center justify-center">
      <div class="bg-elevated border border-text/4 py-8 px-9 w-[clamp(18.75rem,28vw,26.25rem)]">
        <h1 class="text-2xl font-bold text-text/92 m-0 mb-5">Create account</h1>

        <form onsubmit={submit}>
          <div class="flex flex-col gap-1.5 mb-4">
            <label for="username" class="text-[0.6875rem] text-text/45 tracking-[0.08em]">Username</label>
            <input id="username" type="text" autocomplete="username" required bind:value={username} class="field" />
          </div>

          <div class="flex flex-col gap-1.5 mb-4">
            <label for="email" class="text-[0.6875rem] text-text/45 tracking-[0.08em]">Email</label>
            <input id="email" type="email" autocomplete="email" required bind:value={email} class="field" />
          </div>

          <div class="flex flex-col gap-1.5 mb-4">
            <label for="password" class="text-[0.6875rem] text-text/45 tracking-[0.08em]">Password</label>
            <input
              id="password"
              type="password"
              autocomplete="new-password"
              required
              bind:value={password}
              class="field"
            />
          </div>

          <div class="text-[0.6875rem] text-accent min-h-0 mb-1.5">{error}</div>

          <button type="submit" disabled={busy} class="btn-primary">
            {busy ? "..." : "Create account"}
          </button>
        </form>

        <p class="text-[0.75rem] text-text/35 mt-5 mx-0 mb-0 text-center">
          Already have an account? <a href="/login" class="text-accent no-underline">Sign in</a>
        </p>
      </div>
    </div>
  </div>
</div>
