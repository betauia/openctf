import { fmt, esc } from "@lib/utils";
import { markChallengeSolved } from "./filter";
import { timeAgo } from "./utils";

const css = getComputedStyle(document.documentElement);

const CAT_COLORS: Record<string, string> = Object.fromEntries(
  ["web","pwn","crypto","rev","hardware","misc"].map((c) => [c, css.getPropertyValue(`--cat-${c}`).trim()])
);
const DIFF_COLORS: Record<string, string> = Object.fromEntries(
  ["easy","medium","hard","insane"].map((d) => [d, css.getPropertyValue(`--diff-${d}`).trim()])
);
const ptsColor = css.getPropertyValue("--pts-color").trim();

const overlay     = document.getElementById("ch-overlay")!;
const mTitle      = document.getElementById("m-title")!;
const mDesc       = document.getElementById("m-desc")!;
const mDescToggle = document.getElementById("m-desc-toggle") as HTMLButtonElement;
const mCatIcon    = document.getElementById("m-cat-icon")!;
const mStats      = document.getElementById("m-stats")!;
const mConnBlock  = document.getElementById("m-conn-block")!;
const mConnIdle   = document.getElementById("m-conn-idle")!;
const mConnRunning = document.getElementById("m-conn-running")!;
const mConnStart  = document.getElementById("m-conn-start") as HTMLButtonElement;
const mConnHost   = document.getElementById("m-conn-host")!;
const mConnPort   = document.getElementById("m-conn-port")!;
const mFileBlock  = document.getElementById("m-file-block")!;
const mFileList   = document.getElementById("m-file-list")!;
const mFlagForm   = document.getElementById("m-flag-form") as HTMLFormElement;
const mFlagInput  = document.getElementById("m-flag") as HTMLInputElement;
const mFlagBtn    = document.getElementById("m-flag-btn") as HTMLButtonElement;
const mFlagMsg    = document.getElementById("m-flag-msg")!;

const mConnStartInner = mConnStart.innerHTML;
const mFlagBtnInner   = mFlagBtn.innerHTML;

const icon = (name: string) =>
  document.querySelector<HTMLElement>(`#m-icon-cache [data-icon="${name}"]`)?.innerHTML ?? "";

let currentModalId: number | null = null;
let currentInstanceId: string | null = null;
let descExpanded = false;

async function submitFlag(
  flag: string,
  btn: HTMLButtonElement,
  input: HTMLInputElement,
  msg?: HTMLElement,
  onCorrect?: (title: string) => void,
  expectedTitle?: string,
) {
  const origInner = btn.innerHTML;
  btn.disabled = true;
  btn.textContent = "...";
  if (msg) { msg.textContent = ""; msg.className = msg.className.replace(/ ?(correct|wrong)/g, ""); }

  try {
    const res = await fetch("/api/challenges/submit", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ flag }),
    });
    const data = await res.json();

    if (data.correct && (!expectedTitle || data.challenge === expectedTitle)) {
      btn.textContent = `+${data.points}!`;
      btn.classList.add("correct");
      input.value = "";
      if (msg) { msg.textContent = `Correct! +${data.points} points`; msg.classList.add("correct"); }
      const scoreEl = document.getElementById("ch-score-val");
      if (scoreEl) {
        const cur = parseInt(scoreEl.textContent ?? "0") || 0;
        scoreEl.textContent = String(cur + data.points);
        scoreEl.classList.remove("ch-stat-dim");
      }
      onCorrect?.(data.challenge);
    } else {
      btn.textContent = "Wrong";
      btn.classList.add("wrong");
      if (msg) { msg.textContent = "Incorrect flag, try again."; msg.classList.add("wrong"); }
    }
  } catch {
    btn.textContent = "Error";
    btn.classList.add("wrong");
  }

  setTimeout(() => {
    btn.innerHTML = origInner;
    btn.disabled = false;
    btn.classList.remove("correct", "wrong");
  }, 2000);
}

function openModal(challenges: any[], id: number) {
  currentModalId = id;
  const c = challenges.find((x: any) => x.id === id);
  if (!c) return;

  const catColor  = CAT_COLORS[c.category];
  const diffColor = DIFF_COLORS[(c.difficulty ?? "").toLowerCase()];

  mTitle.textContent = c.title;

  const catIconSrc = document.querySelector<HTMLElement>(`#m-cat-icons [data-cat="${c.category}"]`);
  mCatIcon.innerHTML = catIconSrc?.innerHTML ?? c.category[0].toUpperCase();
  mCatIcon.style.cssText = `color:${catColor};background:${catColor}20;border-color:${catColor}40`;

  mDesc.textContent = c.description ?? "No description provided.";
  const descLong = (c.description?.length ?? 0) > 200 || (c.description?.split("\n").length ?? 0) > 4;
  const descWrap = document.getElementById("m-desc-wrap")!;
  if (descLong) {
    mDesc.classList.add("collapsed");
    descWrap.classList.add("collapsed");
    mDescToggle.style.display = "";
    mDescToggle.innerHTML = `${icon("chevron-down")} Show more`;
    descExpanded = false;
  } else {
    mDesc.classList.remove("collapsed");
    descWrap.classList.remove("collapsed");
    mDescToggle.style.display = "none";
  }

  mConnBlock.style.display = (c.connection_info || c.docker_image) ? "" : "none";
  mConnIdle.style.display = "";
  mConnRunning.style.display = "none";
  mConnStart.disabled = false;
  mConnStart.innerHTML = mConnStartInner;
  currentInstanceId = null;

  mFileBlock.style.display = c.files?.length ? "" : "none";
  mFileList.innerHTML = (c.files ?? []).map((f: string) =>
    `<a class="ch-file-item" href="/api/challenges/${c.id}/files/${encodeURIComponent(f)}" download>
      <span class="ch-file-name">${icon("file")} ${esc(f)}</span>
      <span class="ch-file-dl">${icon("download")}</span>
    </a>`
  ).join("");

  mStats.innerHTML = [
    [c.points,          ptsColor,            "POINTS"],
    [fmt(c.difficulty), diffColor,           "DIFFICULTY"],
    [c.author ?? "—",   "var(--muted-text)", "AUTHOR"],
    [c.solves ?? 0,     "var(--muted-text)", "SOLVES"],
  ].map(([val, color, label]) => `
    <div class="ch-modal-stat-cell">
      <div class="ch-modal-stat-val" style="color:${color}">${esc(val)}</div>
      <div class="ch-modal-stat-label">${label}</div>
    </div>`).join("");

  mFlagInput.value = "";
  mFlagMsg.textContent = "";
  mFlagMsg.className = "ch-modal-flag-msg";
  mFlagBtn.innerHTML = mFlagBtnInner;
  mFlagBtn.disabled = false;
  mFlagBtn.className = "";

  document.querySelectorAll<HTMLButtonElement>(".ch-tab").forEach(b =>
    b.classList.toggle("active", b.dataset.tab === "overview")
  );
  document.getElementById("ch-tab-overview")!.style.display = "";
  document.getElementById("ch-tab-solves")!.style.display = "none";

  overlay.classList.add("open");
  mFlagInput.focus();
}

function closeModal() { overlay.classList.remove("open"); }

export function initModal(challenges: any[]) {
  const markSolvedByTitle = (title: string) => {
    const id = challenges.find((c: any) => c.title === title)?.id;
    if (id) markChallengeSolved(id);
  };

  document.querySelectorAll<HTMLTableRowElement>(".ch-row").forEach((row) => {
    row.addEventListener("click", () => openModal(challenges, Number(row.dataset.id)));
  });

  document.getElementById("ch-modal-close")!.addEventListener("click", closeModal);
  overlay.addEventListener("click", (e) => { if (e.target === overlay) closeModal(); });
  document.addEventListener("keydown", (e) => { if (e.key === "Escape") closeModal(); });

  document.querySelectorAll<HTMLButtonElement>(".ch-tab").forEach(btn => {
    btn.addEventListener("click", () => {
      document.querySelectorAll<HTMLButtonElement>(".ch-tab").forEach(b => b.classList.remove("active"));
      btn.classList.add("active");
      const tab = btn.dataset.tab;
      document.getElementById("ch-tab-overview")!.style.display = tab === "overview" ? "" : "none";
      document.getElementById("ch-tab-solves")!.style.display   = tab === "solves"   ? "" : "none";
      if (tab === "solves") loadSolves();
    });
  });

  mDescToggle.addEventListener("click", () => {
    descExpanded = !descExpanded;
    const descWrap = document.getElementById("m-desc-wrap")!;
    mDesc.classList.toggle("collapsed", !descExpanded);
    descWrap.classList.toggle("collapsed", !descExpanded);
    mDescToggle.innerHTML = descExpanded
      ? `${icon("chevron-up")} Show less`
      : `${icon("chevron-down")} Show more`;
  });

  mConnStart.addEventListener("click", async () => {
    const c = challenges.find((x: any) => x.id === currentModalId);
    if (!c) return;
    mConnStart.disabled = true;
    mConnStart.textContent = "…";

    if (c.docker_image) {
      try {
        const res = await fetch("/api/instances", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ challenge_id: c.id }),
        });
        if (!res.ok) throw new Error();
        const inst = await res.json();
        currentInstanceId = inst.id;
        mConnHost.textContent = inst.host_ip ?? "—";
        mConnPort.textContent = inst.host_port ?? "—";
      } catch {
        mConnStart.innerHTML = mConnStartInner;
        mConnStart.disabled = false;
        return;
      }
    } else {
      const parts = c.connection_info.trim().split(/\s+/);
      const port  = parts.findLast((p: string) => /^\d+$/.test(p)) ?? "—";
      const host  = parts.findLast((p: string) => p !== port && !/^nc$|^ncat$|^telnet$/i.test(p)) ?? c.connection_info;
      mConnHost.textContent = host;
      mConnPort.textContent = port;
    }

    mConnIdle.style.display = "none";
    mConnRunning.style.display = "";
  });

  document.getElementById("m-conn-stop")?.addEventListener("click", async () => {
    if (currentInstanceId) {
      try { await fetch(`/api/instances/${currentInstanceId}`, { method: "DELETE" }); } catch {}
      currentInstanceId = null;
    }
    mConnRunning.style.display = "none";
    mConnIdle.style.display = "";
    mConnStart.innerHTML = mConnStartInner;
    mConnStart.disabled = false;
  });

  document.getElementById("m-conn-copy")?.addEventListener("click", () => {
    const c = challenges.find((x: any) => x.id === currentModalId);
    if (!c) return;
    const text = currentInstanceId
      ? `nc ${mConnHost.textContent} ${mConnPort.textContent}`
      : (c.connection_info ?? "");
    navigator.clipboard.writeText(text);
    const btn = document.getElementById("m-conn-copy")!;
    btn.classList.add("copied");
    setTimeout(() => btn.classList.remove("copied"), 1500);
  });

  mFlagForm.addEventListener("submit", (e) => {
    e.preventDefault();
    const flag = mFlagInput.value.trim();
    const openTitle = challenges.find((c: any) => c.id === currentModalId)?.title;
    if (flag) submitFlag(flag, mFlagBtn, mFlagInput, mFlagMsg, markSolvedByTitle, openTitle);
  });

  type SolveEntry = { uid: number; username: string; team: string | null; solved_at: string | null };
  const solvesCache = new Map<number, SolveEntry[]>();

  function renderSolves(panel: HTMLElement, data: SolveEntry[]) {
    if (data.length === 0) { panel.innerHTML = '<div class="ch-modal-placeholder">No solves yet.</div>'; return; }
    panel.innerHTML = `<div class="ch-solves-list">${data.map((s, i) => `
      <div class="ch-solve-row">
        <span class="ch-solve-rank">${String(i + 1).padStart(2, "0")}</span>
        <div class="ch-solve-user">
          <span class="ch-solve-name ch-solve-name-link" data-uid="${s.uid}">${s.username}</span>
          ${s.team ? `<span class="ch-solve-team">${s.team}</span>` : ""}
        </div>
        <span class="ch-solve-time">${timeAgo(s.solved_at)}</span>
      </div>`).join("")}
    </div>`;
    panel.querySelectorAll<HTMLElement>(".ch-solve-name-link").forEach((el) => {
      el.addEventListener("click", () => { window.location.href = `/teams?member=${el.dataset.uid}`; });
    });
  }

  async function loadSolves() {
    const panel = document.getElementById("ch-tab-solves")!;
    if (solvesCache.has(currentModalId)) { renderSolves(panel, solvesCache.get(currentModalId)!); return; }
    panel.innerHTML = '<div class="ch-modal-placeholder">Loading...</div>';
    const res = await fetch(`/api/challenges/${currentModalId}/solves`);
    if (!res.ok) { panel.innerHTML = '<div class="ch-modal-placeholder">Failed to load.</div>'; return; }
    const data: SolveEntry[] = await res.json();
    solvesCache.set(currentModalId, data);
    renderSolves(panel, data);
  }
}
