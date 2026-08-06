import { fmt, esc, exclusiveActive } from "@lib/utils";
import { markChallengeSolved } from "./filter";

// colors
const css = getComputedStyle(document.documentElement);

const CAT_COLORS: Record<string, string> = Object.fromEntries(
  ["web","pwn","crypto","rev","hardware","misc"].map((c) => [c, css.getPropertyValue(`--cat-${c}`).trim()])
);

const DIFF_COLORS: Record<string, string> = Object.fromEntries(
  ["easy","medium","hard","insane"].map((d) => [d, css.getPropertyValue(`--diff-${d}`).trim()])
);

const ptsColor = css.getPropertyValue("--pts-color").trim();

// elements
const overlay      = document.getElementById("ch-overlay")!;
const mTitle       = document.getElementById("m-title")!;
const mCatIcon     = document.getElementById("m-cat-icon")!;
const mStats       = document.getElementById("m-stats")!;

const mDesc        = document.getElementById("m-desc")!;
const mDescWrap    = document.getElementById("m-desc-wrap")!;
const mDescToggle  = document.getElementById("m-desc-toggle") as HTMLButtonElement;

const mConnBlock   = document.getElementById("m-conn-block")!;
const mConnIdle    = document.getElementById("m-conn-idle")!;
const mConnRunning = document.getElementById("m-conn-running")!;
const mConnStart   = document.getElementById("m-conn-start") as HTMLButtonElement;
const mConnHost    = document.getElementById("m-conn-host")!;
const mConnPort    = document.getElementById("m-conn-port")!;

const mFileBlock   = document.getElementById("m-file-block")!;
const mFileList    = document.getElementById("m-file-list")!;

const mFlagForm    = document.getElementById("m-flag-form") as HTMLFormElement;
const mFlagInput   = document.getElementById("m-flag") as HTMLInputElement;
const mFlagBtn     = document.getElementById("m-flag-btn") as HTMLButtonElement;
const mFlagMsg     = document.getElementById("m-flag-msg")!;

const mTabOverview = document.getElementById("ch-tab-overview")!;
const mTabSolves   = document.getElementById("ch-tab-solves")!;

// snapshots
const mConnStartInner = mConnStart.innerHTML;
const mFlagBtnInner   = mFlagBtn.innerHTML;

// helpers
const icon = (name: string) =>
  document.querySelector<HTMLElement>(`#m-icon-cache [data-icon="${name}"]`)?.innerHTML ?? "";

// state
let currentModalId: number | null = null;
let currentInstanceId: string | null = null;
let descExpanded = false;

// instance
async function resolveConn(c: any): Promise<{ host: string; port: string; instanceId?: string } | null> {
  if (c.docker_image) {
    const res = await fetch("/api/instances", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ challenge_id: c.id }),
    });
    if (!res.ok) return null;
    const inst = await res.json();
    return { host: inst.host_ip ?? "—", port: inst.host_port ?? "—", instanceId: inst.id };
  }
  const parts = c.connection_info.trim().split(/\s+/);
  const port  = parts.findLast((p: string) => /^\d+$/.test(p)) ?? "—";
  const host  = parts.findLast((p: string) => p !== port && !/^nc$|^ncat$|^telnet$/i.test(p)) ?? c.connection_info;
  return { host, port };
}

// submission
export async function submitFlag(
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
  if (msg) { msg.textContent = ""; msg.classList.remove("correct", "wrong"); }

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

// open modal
function renderHeader(c: any) {
  const catColor = CAT_COLORS[c.category];
  mTitle.textContent = c.title;
  const catIconSrc = document.querySelector<HTMLElement>(`#m-cat-icons [data-cat="${c.category}"]`);
  mCatIcon.innerHTML = catIconSrc?.innerHTML ?? c.category[0].toUpperCase();
  mCatIcon.style.cssText = `color:${catColor};background:${catColor}20;border-color:${catColor}40`;
}

function renderDesc(c: any) {
  mDesc.textContent = c.description ?? "No description provided.";
  const descLong = (c.description?.length ?? 0) > 200 || (c.description?.split("\n").length ?? 0) > 4;
  mDesc.classList.toggle("collapsed", descLong);
  mDescWrap.classList.toggle("collapsed", descLong);
  mDescToggle.style.display = descLong ? "" : "none";
  if (descLong) { mDescToggle.innerHTML = `${icon("chevron-down")} Show more`; descExpanded = false; }
}

function renderStats(c: any) {
  const diffColor = DIFF_COLORS[(c.difficulty ?? "").toLowerCase()];
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
}

function renderFiles(c: any) {
  mFileBlock.style.display = c.files?.length ? "" : "none";
  mFileList.innerHTML = (c.files ?? []).map((f: string) =>
    `<a class="ch-file-item" href="/api/challenges/${c.id}/files/${encodeURIComponent(f)}" download>
      <span class="ch-file-name">${icon("file")} ${esc(f)}</span>
      <span class="ch-file-dl">${icon("download")}</span>
    </a>`
  ).join("");
}

function resetConn(c: any) {
  mConnBlock.style.display = (c.connection_info || c.docker_image) ? "" : "none";
  mConnIdle.style.display = "";
  mConnRunning.style.display = "none";
  mConnStart.disabled = false;
  mConnStart.innerHTML = mConnStartInner;
  currentInstanceId = null;
}

function resetFlag() {
  mFlagInput.value = "";
  mFlagMsg.textContent = "";
  mFlagMsg.className = "ch-modal-flag-msg";
  mFlagBtn.innerHTML = mFlagBtnInner;
  mFlagBtn.disabled = false;
  mFlagBtn.classList.remove("correct", "wrong");
}

function openModal(challenges: any[], id: number) {
  currentModalId = id;
  const c = challenges.find((x: any) => x.id === id);
  if (!c) return;

  renderHeader(c);
  renderDesc(c);
  renderStats(c);
  renderFiles(c);
  resetConn(c);
  resetFlag();

  exclusiveActive(".ch-tab", document.querySelector<HTMLButtonElement>(".ch-tab[data-tab='overview']")!);
  mTabOverview.style.display = "";
  mTabSolves.style.display = "none";

  overlay.classList.add("open");
  mFlagInput.focus();
}

function closeModal() { overlay.classList.remove("open"); }

// init
export function initModal(challenges: any[]) {
  const markSolvedByTitle = (title: string) => {
    const id = challenges.find((c: any) => c.title === title)?.id;
    if (id) markChallengeSolved(id);
  };

  // rows
  document.querySelectorAll<HTMLTableRowElement>(".ch-row").forEach((row) => {
    row.addEventListener("click", () => openModal(challenges, Number(row.dataset.id)));
  });

  // close
  document.getElementById("ch-modal-close")!.addEventListener("click", closeModal);
  overlay.addEventListener("click", (e) => { if (e.target === overlay) closeModal(); });
  document.addEventListener("keydown", (e) => { if (e.key === "Escape") closeModal(); });

  // tabs
  document.querySelectorAll<HTMLButtonElement>(".ch-tab").forEach(btn => {
    btn.addEventListener("click", () => {
      exclusiveActive(".ch-tab", btn);
      const tab = btn.dataset.tab;
      mTabOverview.style.display = tab === "overview" ? "" : "none";
      mTabSolves.style.display   = tab === "solves"   ? "" : "none";
    });
  });

  // description
  mDescToggle.addEventListener("click", () => {
    descExpanded = !descExpanded;
    mDesc.classList.toggle("collapsed", !descExpanded);
    mDescWrap.classList.toggle("collapsed", !descExpanded);
    mDescToggle.innerHTML = descExpanded
      ? `${icon("chevron-up")} Show less`
      : `${icon("chevron-down")} Show more`;
  });

  // connection
  mConnStart.addEventListener("click", async () => {
    const c = challenges.find((x: any) => x.id === currentModalId);
    if (!c) return;
    mConnStart.disabled = true;
    mConnStart.textContent = "…";

    const conn = await resolveConn(c).catch(() => null);
    if (!conn) {
      mConnStart.innerHTML = mConnStartInner;
      mConnStart.disabled = false;
      return;
    }

    currentInstanceId = conn.instanceId ?? null;
    mConnHost.textContent = conn.host;
    mConnPort.textContent = conn.port;
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

  const mConnCopy = document.getElementById("m-conn-copy")!;
  mConnCopy.addEventListener("click", () => {
    const c = challenges.find((x: any) => x.id === currentModalId);
    if (!c) return;
    const text = currentInstanceId
      ? `nc ${mConnHost.textContent} ${mConnPort.textContent}`
      : (c.connection_info ?? "");
    navigator.clipboard.writeText(text);
    mConnCopy.classList.add("copied");
    setTimeout(() => mConnCopy.classList.remove("copied"), 1500);
  });

  // flag
  mFlagForm.addEventListener("submit", (e) => {
    e.preventDefault();
    const flag = mFlagInput.value.trim();
    const openTitle = challenges.find((c: any) => c.id === currentModalId)?.title;
    if (flag) submitFlag(flag, mFlagBtn, mFlagInput, mFlagMsg, markSolvedByTitle, openTitle);
  });
}
