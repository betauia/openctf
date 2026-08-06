import { initFilter, markChallengeSolved } from "./filter";
import { initModal, submitFlag } from "./modal";

const { challenges, solvedIds } = JSON.parse(document.getElementById("ch-data")!.textContent!);

initFilter(solvedIds);
initModal(challenges, solvedIds);

// global flagbar
const flagForm  = document.getElementById("ch-flag-form") as HTMLFormElement;
const flagInput = document.getElementById("ch-flag") as HTMLInputElement;
const flagBtn   = document.getElementById("ch-flag-btn") as HTMLButtonElement;

flagForm?.addEventListener("submit", (e) => {
  e.preventDefault();
  const flag = flagInput.value.trim();
  
  if (flag) submitFlag(flag, flagBtn, flagInput, undefined, (title) => {
    const id = challenges.find((c: any) => c.title === title)?.id;
    if (id) markChallengeSolved(id);
  });
});
