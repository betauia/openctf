import { initFilter, markChallengeSolved } from "./filter";
import { initModal } from "./modal";

const { challenges } = JSON.parse(document.getElementById("ch-data")!.textContent!);

initFilter();
initModal(challenges);

// global flag bar
const flagForm  = document.getElementById("ch-flag-form") as HTMLFormElement;
const flagInput = document.getElementById("ch-flag") as HTMLInputElement;
const flagBtn   = document.getElementById("ch-flag-btn") as HTMLButtonElement;

flagForm?.addEventListener("submit", async (e) => {
  e.preventDefault();
  const flag = flagInput.value.trim();
  if (!flag) return;

  const origInner = flagBtn.innerHTML;
  flagBtn.disabled = true;
  flagBtn.textContent = "...";

  try {
    const res = await fetch("/api/challenges/submit", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ flag }),
    });
    const data = await res.json();

    if (data.correct) {
      flagBtn.textContent = `+${data.points}!`;
      flagBtn.classList.add("correct");
      flagInput.value = "";
      const scoreEl = document.getElementById("ch-score-val");
      if (scoreEl) {
        const cur = parseInt(scoreEl.textContent ?? "0") || 0;
        scoreEl.textContent = String(cur + data.points);
        scoreEl.classList.remove("ch-stat-dim");
      }
      const id = challenges.find((c: any) => c.title === data.challenge)?.id;
      if (id) markChallengeSolved(id);
    } else {
      flagBtn.textContent = "Wrong";
      flagBtn.classList.add("wrong");
    }
  } catch {
    flagBtn.textContent = "Error";
    flagBtn.classList.add("wrong");
  }

  setTimeout(() => {
    flagBtn.innerHTML = origInner;
    flagBtn.disabled = false;
    flagBtn.classList.remove("correct", "wrong");
  }, 2000);
});
