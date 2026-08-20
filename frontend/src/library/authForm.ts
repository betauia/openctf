export function bindAuthForm(endpoint: string, fallbackError: string) {
  const form = document.getElementById("auth-form") as HTMLFormElement;
  const btn = document.getElementById("auth-btn") as HTMLButtonElement;
  const err = document.getElementById("auth-error")!;
  const originalLabel = btn.textContent;

  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    btn.disabled = true;
    btn.textContent = "...";
    err.textContent = "";

    const res = await fetch(endpoint, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(Object.fromEntries(new FormData(form))),
    });

    if (res.ok) {
      window.location.href = "/challenges";
    } else {
      const body = await res.json().catch(() => ({}));
      err.textContent = body.detail ?? fallbackError;
      btn.disabled = false;
      btn.textContent = originalLabel;
    }
  });
}
