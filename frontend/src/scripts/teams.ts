import { exclusiveActive, flashCopy, timeAgo, iconFrom } from "@library/utils";

// data
const { teams, inspectMemberId } = JSON.parse(document.getElementById("tm-data")!.textContent!);

// helpers
const icon = iconFrom("tm-icon-cache");

async function post(url: string, body?: unknown) {
  return fetch(url, {
    method: "POST",
    headers: body !== undefined ? { "Content-Type": "application/json" } : {},
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
}

// forms
function bindForm(formId: string, errorId: string, handler: (fd: FormData) => Promise<void>) {
  const form = document.getElementById(formId) as HTMLFormElement | null;
  if (!form) return;
  const err = document.getElementById(errorId)!;
  const btn = form.querySelector("button[type=submit]") as HTMLButtonElement;
  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    btn.disabled = true;
    err.textContent = "";
    try {
      await handler(new FormData(form));
    } catch (ex: any) {
      err.textContent = ex.message;
      btn.disabled = false;
    }
  });
}

bindForm("tm-create-form", "tm-create-error", async (fd) => {
  const res = await post("/api/teams", { name: fd.get("name") });
  if (!res.ok) throw new Error((await res.json()).detail ?? "Failed to create team");
  location.reload();
});

bindForm("tm-join-form", "tm-join-error", async (fd) => {
  const res = await post("/api/teams/join", { invite_code: fd.get("invite_code") });
  if (!res.ok) throw new Error((await res.json()).detail ?? "Invalid invite code");
  location.reload();
});

document.getElementById("tm-leave-btn")?.addEventListener("click", async () => {
  const res = await post("/api/teams/leave");
  if (!res.ok) {
    const err = document.getElementById("tm-error");
    if (err) err.textContent = (await res.json()).detail ?? "Failed";
    return;
  }
  location.reload();
});

// copy
document.getElementById("tm-copy-btn")?.addEventListener("click", (e) => {
  const code = document.getElementById("tm-invite-code")?.textContent ?? "";
  flashCopy(e.currentTarget as HTMLButtonElement, code);
});

// tabs
document.querySelectorAll<HTMLButtonElement>(".tm-tab").forEach((btn) => {
  btn.addEventListener("click", () => {
    exclusiveActive(".tm-tab", btn);
    const tab = btn.dataset.tab!;
    ["team", "leaderboard"].forEach((t) => {
      const el = document.getElementById(`tab-${t}`);
      if (el) el.style.display = t === tab ? "" : "none";
    });
  });
});

// member
type SolveRow = { challenge: string; category: string; points: number; solved_at: string | null };
type MemberData = { username: string; score: number; solves: SolveRow[] };

async function fetchMember(id: string): Promise<MemberData | null> {
  const res = await fetch(`/api/teams/members/${id}`);
  return res.ok ? res.json() : null;
}

const numCls = "text-[11px] text-text/18 w-9";
const nameCls = "text-muted-text font-medium";
const monoCls = "text-[12px] text-text/45 text-center";
const ptsCls = "font-bold text-pts text-center";
const emptyCls = "text-center py-20 text-text/18 text-[13px]";
const tableWrapCls = "flex-1 overflow-y-auto p-4";
const thBase = "py-2 px-3 text-[11px] tracking-[0.12em] text-text/55";
const thCls = `${thBase} text-left`;
const thCenterCls = `${thBase} text-center`;
const avatarCls = "w-7 h-7 bg-accent/15 border border-accent/30 flex items-center justify-center text-[12px] font-bold text-accent shrink-0";
const detailHeaderCls = "flex items-center gap-3.5 py-4.5 px-6 bg-elevated border-b border-border-subtle shrink-0";
const backBtnCls = "flex items-center gap-1.25 bg-transparent border-none text-text/30 text-[11px] tracking-[0.06em] cursor-pointer p-0 mr-1.5 transition-colors duration-100 hover:text-muted-text";

function memberSolvesHTML(data: MemberData): string {
  if (data.solves.length === 0) return `<div class="${emptyCls}">No solves yet</div>`;
  const rows = data.solves.map((s, i) =>
    `<tr>
      <td class="py-2.5 px-3 text-[13px] ${numCls}">${String(i + 1).padStart(2, "0")}</td>
      <td class="py-2.5 px-3 text-[13px] ${nameCls}">${s.challenge}</td>
      <td class="py-2.5 px-3 text-[13px] text-center"><span class="inline-flex items-center gap-1 py-0.5 px-1.75 text-[10px] tracking-[0.07em] cat-${s.category}">${s.category}</span></td>
      <td class="py-2.5 px-3 text-[13px] ${ptsCls}">${s.points}</td>
      <td class="py-2.5 px-3 text-[13px] ${monoCls}">${timeAgo(s.solved_at)}</td>
    </tr>`
  ).join("");
  return `<div class="${tableWrapCls}">
    <table class="w-full border-separate border-spacing-x-0 border-spacing-y-0.5">
      <thead><tr><th class="${thCls}">#</th><th class="${thCls}">Challenge</th><th class="${thCenterCls}">Category</th><th class="${thCenterCls}">Points</th><th class="${thCenterCls}">Solved</th></tr></thead>
      <tbody>${rows}</tbody>
    </table>
  </div>`;
}

function memberHeader(data: MemberData, backLabel: string, backBtnId: string): string {
  return `<div class="${detailHeaderCls}">
    <button id="${backBtnId}" class="${backBtnCls}">${icon("arrow-left")} ${backLabel}</button>
    <div class="${avatarCls} w-10 h-10 text-base">${data.username[0].toUpperCase()}</div>
    <div>
      <div class="text-base font-bold text-text">${data.username}</div>
      <div class="text-[11px] text-text/35 mt-0.75">${data.score} pts · ${data.solves.length} solved</div>
    </div>
  </div>`;
}

function activityHTML(activity: any[]): string {
  const entries = activity.map(e => `
    <div class="flex items-start gap-3 py-2 px-6 border-b border-border-subtle last:border-b-0">
      <div class="${avatarCls} w-6 h-6 text-[10px] mt-px">${e.username[0].toUpperCase()}</div>
      <div class="flex flex-col gap-0.75">
        <div class="flex items-center flex-wrap gap-1.5 text-[12px]">
          <span class="text-muted-text font-semibold">${e.username}</span>
          <span class="text-text/30">solved</span>
          <span class="text-[11px] py-px px-1.75 cat-${e.category}">${e.challenge}</span>
          <span class="text-[11px] font-bold text-pts">+${e.points}</span>
        </div>
        <div class="text-[10px] text-text/20 tracking-[0.04em]">${timeAgo(e.solved_at)}</div>
      </div>
    </div>`).join("");
  return `<div class="border-t border-border-subtle flex flex-col">
    <div class="text-[9px] tracking-[0.14em] text-text/25 pt-2.5 px-6 pb-1.5 shrink-0">Recent Activity</div>
    <div class="max-h-60 overflow-y-auto">${entries}</div>
  </div>`;
}

function renderMemberDetail(container: HTMLElement, data: MemberData, backLabel: string, onBack: () => void) {
  container.innerHTML = memberHeader(data, backLabel, "detail-back-btn") + memberSolvesHTML(data);
  document.getElementById("detail-back-btn")!.addEventListener("click", onBack);
}

function showMemberProfile(data: MemberData) {
  const ws = document.getElementById("tm-workspace")!;
  const det = document.getElementById("tm-member-detail")!;
  renderMemberDetail(det, data, "Back", () => { det.style.display = "none"; ws.style.display = ""; });
  ws.style.display = "none";
  det.style.display = "";
}

document.querySelectorAll<HTMLTableRowElement>(".tm-member-row").forEach((row) => {
  if (!row.dataset.id) return;
  row.style.cursor = "pointer";
  row.addEventListener("click", async () => {
    const data = await fetchMember(row.dataset.id!);
    if (data) showMemberProfile(data);
  });
});

// leaderboard
const lb = document.getElementById("tab-leaderboard")!;
const lbHTML = lb.innerHTML;

function teamDetailHTML(team: any, rank: number, sorted: any[]): string {
  const rows = sorted.map((m: any, i: number) =>
    `<tr class="tm-member-row" data-id="${m.id}" style="cursor:pointer">
      <td class="py-2.5 px-3 text-[13px] ${numCls}">${String(i + 1).padStart(2, "0")}</td>
      <td class="py-2.5 px-3 text-[13px]">
        <div class="flex items-center gap-2.5">
          <div class="${avatarCls}">${m.username[0].toUpperCase()}</div>
          <span class="${nameCls}">${m.username}</span>
        </div>
      </td>
      <td class="py-2.5 px-3 text-[13px] ${ptsCls}">${m.score}</td>
      <td class="py-2.5 px-3 text-[13px] ${monoCls}">${m.solves ?? 0}</td>
    </tr>`
  ).join("");
  return `<div class="${detailHeaderCls}">
    <button id="lb-back-btn" class="${backBtnCls}">${icon("arrow-left")} Back</button>
    <div class="${avatarCls} w-10 h-10 text-base">${team.name[0].toUpperCase()}</div>
    <div>
      <div class="text-base font-bold text-text">${team.name}</div>
      <div class="text-[11px] text-text/35 mt-0.75">${team.score} pts · Rank #${rank}</div>
    </div>
  </div>
  <div class="${tableWrapCls}" style="max-height:280px;flex:none;">
    <table class="w-full border-separate border-spacing-x-0 border-spacing-y-0.5">
      <thead><tr><th class="${thCls}">#</th><th class="${thCls}">Member</th><th class="${thCenterCls}">Score</th><th class="${thCenterCls}">Solves</th></tr></thead>
      <tbody>${rows}</tbody>
    </table>
  </div>`;
}

async function inspectTeam(team: any, rank: number) {
  const sorted = [...team.members].sort((a: any, b: any) => b.score - a.score);
  lb.innerHTML = teamDetailHTML(team, rank, sorted);

  document.getElementById("lb-back-btn")!.addEventListener("click", bindLeaderboardRows);

  lb.querySelectorAll<HTMLTableRowElement>(".tm-member-row").forEach((row) => {
    row.addEventListener("click", async () => {
      const data = await fetchMember(row.dataset.id!);
      if (data) inspectMember(data, team, rank);
    });
  });

  const actRes = await fetch(`/api/teams/${team.id}/activity`);
  if (actRes.ok && lb.contains(document.getElementById("lb-back-btn"))) {
    const activity: any[] = await actRes.json();
    if (activity.length > 0) lb.insertAdjacentHTML("beforeend", activityHTML(activity));
  }
}

function inspectMember(data: MemberData, team: any, rank: number) {
  renderMemberDetail(lb, data, "Back to team", () => inspectTeam(team, rank));
}

function bindLeaderboardRows() {
  lb.innerHTML = lbHTML;
  lb.querySelectorAll<HTMLTableRowElement>(".tm-row").forEach((row) => {
    row.addEventListener("click", () => {
      if (row.classList.contains("tm-mine")) {
        document.querySelector<HTMLButtonElement>('[data-tab="team"]')!.click();
        return;
      }
      inspectTeam(JSON.parse(row.dataset.team!), parseInt(row.dataset.rank!));
    });
  });
}

bindLeaderboardRows();

// deep-link
if (inspectMemberId) {
  const team = teams?.find((t: any) => t.members.some((m: any) => m.id === inspectMemberId));
  if (team) {
    document.querySelector<HTMLButtonElement>('[data-tab="leaderboard"]')!.click();
    fetchMember(String(inspectMemberId)).then(
      (data) => data && inspectMember(data, team, teams.indexOf(team) + 1),
    );
  }
}
