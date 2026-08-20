import { CAT_SLUGS, DIFF_SLUGS } from "@library/categories";
import { esc, exclusiveActive, flashCopy, fmt, iconFrom, timeAgo } from "@library/utils";
import { markChallengeSolved } from "./filter";

// colors
const css = getComputedStyle(document.documentElement);

const CAT_COLORS: Record<string, string> = Object.fromEntries(
  CAT_SLUGS.map((c) => [c, css.getPropertyValue(`--cat-${c}`).trim()]),
);

const DIFF_COLORS: Record<string, string> = Object.fromEntries(
  DIFF_SLUGS.map((d) => [d, css.getPropertyValue(`--diff-${d}`).trim()]),
);

const ptsColor = css.getPropertyValue("--pts-color").trim();

// elements
const $ = <T extends HTMLElement = HTMLElement>(id: string) =>
  document.getElementById(id) as T;

const overlay = $("ch-overlay");
const mTitle = $("m-title");
const mCatIcon = $("m-cat-icon");
const mStats = $("m-stats");

const mDesc = $("m-desc");
const mDescWrap = $("m-desc-wrap");
const mDescToggle = $<HTMLButtonElement>("m-desc-toggle");

const mConnBlock = $("m-conn-block");
const mConnIdle = $("m-conn-idle");
const mConnRunning = $("m-conn-running");
const mConnStart = $<HTMLButtonElement>("m-conn-start");
const mConnHost = $("m-conn-host");
const mConnPort = $("m-conn-port");

const mFileBlock = $("m-file-block");
const mFileList = $("m-file-list");

const mFlagForm = $<HTMLFormElement>("m-flag-form");
const mFlagInput = $<HTMLInputElement>("m-flag");
const mFlagBtn = $<HTMLButtonElement>("m-flag-btn");
const mFlagMsg = $("m-flag-msg");

const mTabOverview = $("ch-tab-overview");
const mTabSolves = $("ch-tab-solves");
const mSolvedBanner = $("m-solved-banner");
const mConnStop = $("m-conn-stop");

// snapshots
const mConnStartInner = mConnStart.innerHTML;
const mFlagBtnInner = mFlagBtn.innerHTML;

// helpers
const icon = iconFrom("m-icon-cache");

// state
let challenges: any[] = [];
let currentModalId: number | null = null;
let currentInstanceId: string | null = null;
let descExpanded = false;
let solvedSet = new Set<number>();
let pollInterval: ReturnType<typeof setInterval> | null = null;

function showSolvedBanner() {
  mSolvedBanner.style.display = "";
}
function hideSolvedBanner() {
  mSolvedBanner.style.display = "none";
}

function stopPoll() {
  if (pollInterval) {
    clearInterval(pollInterval);
    pollInterval = null;
  }
}

function startPoll() {
  stopPoll();
  pollInterval = setInterval(async () => {
    const id = currentModalId;
    if (!id || solvedSet.has(id)) {
      stopPoll();
      return;
    }
    const res = await fetch(`/api/challenges/${id}/solves`);
    if (res.ok && (await res.json()).length > 0) {
      solvedSet.add(id);
      markChallengeSolved(id);
      showSolvedBanner();
      stopPoll();
    }
  }, 5000);
}

// instance
async function resolveConn(
  c: any,
): Promise<{ host: string; port: string; instanceId?: string } | null> {
  if (c.docker_image) {
    const res = await fetch("/api/instances", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ challenge_id: c.id }),
    });
    if (!res.ok) return null;
    const inst = await res.json();
    return {
      host: inst.host_ip ?? "—",
      port: inst.host_port ?? "—",
      instanceId: inst.id,
    };
  }

  // connection_info is free text set by the challenge author (e.g. "nc host port", "ncat ...")
  const parts = c.connection_info.trim().split(/\s+/);
  const port = parts.findLast((p: string) => /^\d+$/.test(p)) ?? "—";
  const host =
    parts.findLast(
      (p: string) => p !== port && !/^nc$|^ncat$|^telnet$/i.test(p),
    ) ?? c.connection_info;
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
  if (msg) {
    msg.textContent = "";
    msg.classList.remove("correct", "wrong");
  }

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
      if (msg) {
        msg.textContent = `Correct! +${data.points} points`;
        msg.classList.add("correct");
      }
      onCorrect?.(data.challenge);
    } else {
      btn.textContent = "Wrong";
      btn.classList.add("wrong");
      if (msg) {
        msg.textContent = "Incorrect flag, try again.";
        msg.classList.add("wrong");
      }
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
  const catIconSrc = document.querySelector<HTMLElement>(
    `#m-cat-icons [data-cat="${c.category}"]`,
  );
  mCatIcon.innerHTML = catIconSrc?.innerHTML ?? "";
  mCatIcon.style.cssText = `color:${catColor};background:${catColor}20;border-color:${catColor}40`;
}

function setDescCollapsed(collapsed: boolean) {
  mDesc.classList.toggle("collapsed", collapsed);
  mDescWrap.classList.toggle("collapsed", collapsed);
}

function renderDesc(c: any) {
  mDesc.textContent = c.description ?? "No description provided.";
  const descLong =
    (c.description?.length ?? 0) > 200 ||
    (c.description?.split("\n").length ?? 0) > 4;
  setDescCollapsed(descLong);
  mDescToggle.style.display = descLong ? "" : "none";
  if (descLong) {
    mDescToggle.innerHTML = `${icon("chevron-down")} Show more`;
    descExpanded = false;
  }
}

function renderStats(c: any) {
  const diffColor = DIFF_COLORS[(c.difficulty ?? "").toLowerCase()];
  mStats.innerHTML = [
    [c.points, ptsColor, "POINTS"],
    [fmt(c.difficulty), diffColor, "DIFFICULTY"],
    [c.author ?? "—", "var(--muted-text)", "AUTHOR"],
    [c.solves ?? 0, "var(--muted-text)", "SOLVES"],
  ]
    .map(
      ([val, color, label]) => `
    <div class="p-4 text-center bg-elevated border-r border-b border-border-subtle [&:nth-child(even)]:border-r-0 [&:nth-last-child(-n+2)]:border-b-0">
      <div class="mb-1 text-[17px] font-bold" style="color:${color}">${esc(val)}</div>
      <div class="text-[9px] tracking-[0.14em] uppercase text-text/28">${label}</div>
    </div>`,
    )
    .join("");
}

function renderFiles(c: any) {
  mFileBlock.style.display = c.files?.length ? "" : "none";
  mFileList.innerHTML = (c.files ?? [])
    .map(
      (f: string) =>
        `<a class="group flex items-center justify-between rounded py-1.75 px-2.5 text-xs no-underline border border-border-subtle bg-text/4 text-text/60 transition-colors duration-150 hover:bg-text/8 hover:text-text/90" href="/api/challenges/${c.id}/files/${encodeURIComponent(f)}" download>
      <span class="inline-flex items-center gap-1.5 font-mono">${icon("file")} ${esc(f)}</span>
      <span class="flex items-center text-text/30 transition-colors duration-150 group-hover:text-text/70">${icon("download")}</span>
    </a>`,
    )
    .join("");
}

type SolveEntry = { uid: number; username: string; team: string | null; solved_at: string | null };

function solveRowHTML(s: SolveEntry, i: number): string {
  return `<div class="ch-solve-row">
    <span class="ch-solve-rank">${String(i + 1).padStart(2, "0")}</span>
    <div class="ch-solve-user">
      <span class="ch-solve-name ch-solve-name-link" data-uid="${s.uid}">${esc(s.username)}</span>
      ${s.team ? `<span class="ch-solve-team">${esc(s.team)}</span>` : ""}
    </div>
    <span class="ch-solve-time">${timeAgo(s.solved_at)}</span>
  </div>`;
}

function resetConnUI() {
  mConnIdle.style.display = "";
  mConnRunning.style.display = "none";
  mConnStart.disabled = false;
  mConnStart.innerHTML = mConnStartInner;
}

function resetConn(c: any) {
  mConnBlock.style.display = c.connection_info || c.docker_image ? "" : "none";
  resetConnUI();
  currentInstanceId = null;
}

function resetFlag() {
  mFlagInput.value = "";
  mFlagMsg.textContent = "";
  mFlagMsg.classList.remove("correct", "wrong");
  mFlagBtn.innerHTML = mFlagBtnInner;
  mFlagBtn.disabled = false;
  mFlagBtn.classList.remove("correct", "wrong");
}

function openModal(id: number) {
  currentModalId = id;
  const c = findChallenge(id);
  if (!c) return;

  renderHeader(c);
  renderDesc(c);
  renderStats(c);
  renderFiles(c);
  resetConn(c);
  resetFlag();

  exclusiveActive(
    ".ch-tab",
    document.querySelector<HTMLButtonElement>(".ch-tab[data-tab='overview']")!,
  );
  mTabOverview.style.display = "";
  mTabSolves.style.display = "none";

  if (solvedSet.has(id)) showSolvedBanner();
  else {
    hideSolvedBanner();
    startPoll();
  }

  overlay.classList.remove("hidden");
  overlay.classList.add("flex");
  mFlagInput.focus();
}

function closeModal() {
  overlay.classList.remove("flex");
  overlay.classList.add("hidden");
  stopPoll();
}

function findChallenge(id: number | null) {
  return challenges.find((c: any) => c.id === id);
}

// init
export function initModal(chs: any[], initialSolvedIds: number[]) {
  challenges = chs;
  solvedSet = new Set(initialSolvedIds);

  const markSolvedByTitle = (title: string) => {
    const id = challenges.find((c: any) => c.title === title)?.id;
    if (!id) return;
    solvedSet.add(id);
    markChallengeSolved(id);
    if (id === currentModalId) {
      showSolvedBanner();
      stopPoll();
    }
  };

  // rows
  document.querySelectorAll<HTMLTableRowElement>(".ch-row").forEach((row) => {
    row.addEventListener("click", () => openModal(Number(row.dataset.id)));
  });

  // close
  $("ch-modal-close").addEventListener("click", closeModal);
  overlay.addEventListener("click", (e) => {
    if (e.target === overlay) closeModal();
  });
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape") closeModal();
  });

  // solves tab
  const solvesCache = new Map<number, SolveEntry[]>();

  mTabSolves.addEventListener("click", (e) => {
    const el = (e.target as HTMLElement).closest<HTMLElement>(
      ".ch-solve-name-link",
    );
    if (el) window.location.href = `/teams?member=${el.dataset.uid}`;
  });

  async function loadSolves() {
    const id = currentModalId!;
    if (!solvesCache.has(id)) {
      mTabSolves.innerHTML = '<div class="text-center py-12 text-[13px] text-text/22">Loading...</div>';
      const res = await fetch(`/api/challenges/${id}/solves`);
      if (!res.ok) {
        mTabSolves.innerHTML =
          '<div class="text-center py-12 text-[13px] text-text/22">Failed to load.</div>';
        return;
      }
      solvesCache.set(id, await res.json());
    }
    const data = solvesCache.get(id)!;
    mTabSolves.innerHTML = data.length === 0
      ? '<div class="text-center py-12 text-[13px] text-text/22">No solves yet.</div>'
      : `<div class="ch-solves-list">${data.map(solveRowHTML).join("")}</div>`;
  }

  // tabs
  document.querySelectorAll<HTMLButtonElement>(".ch-tab").forEach((btn) => {
    btn.addEventListener("click", () => {
      exclusiveActive(".ch-tab", btn);
      const tab = btn.dataset.tab;
      mTabOverview.style.display = tab === "overview" ? "" : "none";
      mTabSolves.style.display = tab === "solves" ? "" : "none";
      if (tab === "solves") loadSolves();
    });
  });

  // description
  mDescToggle.addEventListener("click", () => {
    descExpanded = !descExpanded;
    setDescCollapsed(!descExpanded);
    mDescToggle.innerHTML = descExpanded
      ? `${icon("chevron-up")} Show less`
      : `${icon("chevron-down")} Show more`;
  });

  // connection
  mConnStart.addEventListener("click", async () => {
    const c = findChallenge(currentModalId);
    if (!c) return;
    mConnStart.disabled = true;
    mConnStart.textContent = "…";

    const conn = await resolveConn(c).catch(() => null);
    if (!conn) {
      resetConnUI();
      return;
    }

    currentInstanceId = conn.instanceId ?? null;
    mConnHost.textContent = conn.host;
    mConnPort.textContent = conn.port;
    mConnIdle.style.display = "none";
    mConnRunning.style.display = "";
  });

  mConnStop.addEventListener("click", async () => {
    if (currentInstanceId) {
      try {
        await fetch(`/api/instances/${currentInstanceId}`, {
          method: "DELETE",
        });
      } catch {}
      currentInstanceId = null;
    }
    resetConnUI();
  });

  const mConnCopy = $("m-conn-copy");
  mConnCopy.addEventListener("click", () => {
    const c = findChallenge(currentModalId);
    if (!c) return;
    const text = currentInstanceId
      ? `nc ${mConnHost.textContent} ${mConnPort.textContent}`
      : (c.connection_info ?? "");
    flashCopy(mConnCopy, text);
  });

  // flag
  mFlagForm.addEventListener("submit", (e) => {
    e.preventDefault();
    const flag = mFlagInput.value.trim();
    const openTitle = findChallenge(currentModalId)?.title;
    if (flag)
      submitFlag(
        flag,
        mFlagBtn,
        mFlagInput,
        mFlagMsg,
        markSolvedByTitle,
        openTitle,
      );
  });
}
