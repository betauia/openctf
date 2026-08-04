const tbody = document.querySelector<HTMLTableSectionElement>("#ch-table tbody")!;
const solvedSep   = document.getElementById("ch-solved-sep")!;
const solvedArrow = document.getElementById("ch-solved-arrow")!;
const solvedCount = document.getElementById("ch-solved-count")!;

const solvedIds = new Set<number>(
  JSON.parse(document.getElementById("ch-data")!.textContent!).solvedIds
);

let solvedCollapsed = localStorage.getItem("openctf_solved_collapsed") === "true";

const rowById = (id: number) =>
  tbody.querySelector<HTMLTableRowElement>(`.ch-row[data-id="${id}"]`);

function updateSolvedUI() {
  solvedCount.textContent = solvedIds.size > 0 ? `(${solvedIds.size})` : "";
  solvedArrow.classList.toggle("collapsed", solvedCollapsed);
}

let activeCat   = "all";
let searchQuery = "";

function filterRows() {
  let visibleSolved = 0;
  document.querySelectorAll<HTMLTableRowElement>("#ch-table tbody .ch-row").forEach((row) => {
    const isSolved = row.classList.contains("ch-solved");
    const match =
      (activeCat === "all" || row.dataset.cat === activeCat) &&
      (row.dataset.name ?? "").includes(searchQuery);
    if (isSolved) {
      row.style.display = match && !solvedCollapsed ? "" : "none";
      if (match) visibleSolved++;
    } else {
      row.style.display = match ? "" : "none";
    }
  });
  const anyMatchSolved = solvedCollapsed && [...solvedIds].some((id) => {
    const r = rowById(id);
    return r && (activeCat === "all" || r.dataset.cat === activeCat) && (r.dataset.name ?? "").includes(searchQuery);
  });
  solvedSep.style.display = visibleSolved > 0 || anyMatchSolved ? "" : "none";
}

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

export function initFilter() {
  solvedIds.forEach((id) => {
    const row = rowById(id);
    if (row) { row.classList.add("ch-solved"); tbody.appendChild(row); }
  });
  if (solvedIds.size > 0) tbody.insertBefore(solvedSep, tbody.querySelector(".ch-solved")!);
  updateSolvedUI();
  filterRows();

  document.getElementById("ch-solved-toggle")!.addEventListener("click", () => {
    solvedCollapsed = !solvedCollapsed;
    localStorage.setItem("openctf_solved_collapsed", String(solvedCollapsed));
    updateSolvedUI();
    filterRows();
  });

  document.querySelectorAll<HTMLButtonElement>(".ch-cat").forEach((btn) => {
    btn.addEventListener("click", () => {
      document.querySelectorAll<HTMLButtonElement>(".ch-cat").forEach((b) => b.classList.remove("active"));
      btn.classList.add("active");
      activeCat = btn.dataset.cat ?? "all";
      filterRows();
    });
  });

  const searchInput = document.getElementById("ch-search") as HTMLInputElement;
  searchInput?.addEventListener("input", () => {
    searchQuery = searchInput.value.toLowerCase();
    filterRows();
  });
}
