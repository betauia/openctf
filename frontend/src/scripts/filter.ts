import { exclusiveActive } from "@library/utils";

// elements
const tbody       = document.querySelector<HTMLTableSectionElement>("#ch-table tbody")!;
const solvedSep   = document.getElementById("ch-solved-sep")!;
const solvedArrow = document.getElementById("ch-solved-arrow")!;
const solvedCount = document.getElementById("ch-solved-count")!;

// state
export const solvedIds = new Set<number>();
let solvedCollapsed = localStorage.getItem("openctf_solved_collapsed") === "true";
let activeCat   = "all";
let searchQuery = "";

// helpers
const rowById = (id: number) =>
  tbody.querySelector<HTMLTableRowElement>(`.ch-row[data-id="${id}"]`);

function updateSolvedUI() {
  solvedCount.textContent = solvedIds.size > 0 ? `(${solvedIds.size})` : "";
  solvedArrow.classList.toggle("collapsed", solvedCollapsed);
}

// filter
function matches(row: HTMLTableRowElement) {
  return (activeCat === "all" || row.dataset.cat === activeCat) &&
    (row.dataset.name ?? "").includes(searchQuery);
}

export function filterRows() {
  let visibleSolved = 0;
  tbody.querySelectorAll<HTMLTableRowElement>(".ch-row").forEach((row) => {
    const isSolved = row.classList.contains("ch-solved");
    const match = matches(row);
    row.style.display = match && (!isSolved || !solvedCollapsed) ? "" : "none";
    if (isSolved && match) visibleSolved++;
  });
  const anyMatchSolved = solvedCollapsed &&
    [...solvedIds].some((id) => { const r = rowById(id); return r && matches(r); });
  solvedSep.style.display = visibleSolved > 0 || anyMatchSolved ? "" : "none";
}

// solved
export function markChallengeSolved(id: number) {
  if (solvedIds.has(id)) return;
  solvedIds.add(id);
  const row = rowById(id);
  if (row) {
    row.classList.add("ch-solved");
    if (!tbody.contains(solvedSep)) tbody.appendChild(solvedSep);
    tbody.appendChild(row);
  }
  updateSolvedUI();
  filterRows();
}

// init
export function initFilter(ids: number[]) {
  ids.forEach(id => solvedIds.add(id));
  solvedIds.forEach((id) => {
    const row = rowById(id);
    if (row) { row.classList.add("ch-solved"); tbody.appendChild(row); }
  });
  if (solvedIds.size > 0) tbody.insertBefore(solvedSep, tbody.querySelector(".ch-solved")!);
  updateSolvedUI();
  filterRows();

  // collapse
  document.getElementById("ch-solved-toggle")!.addEventListener("click", () => {
    solvedCollapsed = !solvedCollapsed;
    localStorage.setItem("openctf_solved_collapsed", String(solvedCollapsed));
    updateSolvedUI();
    filterRows();
  });

  // categories
  document.querySelectorAll<HTMLButtonElement>(".ch-cat").forEach((btn) => {
    btn.addEventListener("click", () => {
      exclusiveActive(".ch-cat", btn);
      activeCat = btn.dataset.cat ?? "all";
      filterRows();
    });
  });

  // search
  const searchInput = document.getElementById("ch-search") as HTMLInputElement;
  searchInput?.addEventListener("input", () => {
    searchQuery = searchInput.value.toLowerCase();
    filterRows();
  });
}
