import { timeAgo } from "./utils";

const icon = (name: string) =>
  document.querySelector<HTMLElement>(`#tm-icon-cache [data-icon="${name}"]`)?.innerHTML ?? "";

type SolveRow = { challenge: string; category: string; points: number; solved_at: string | null };
type MemberData = { username: string; score: number; solves: SolveRow[] };

async function post(url: string, body?: unknown) {
  return fetch(url, {
    method: "POST",
    headers: body !== undefined ? { "Content-Type": "application/json" } : {},
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
}

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

function memberSolvesHTML(data: MemberData): string {
  if (data.solves.length === 0) return '<div class="ch-empty">No solves yet</div>';
  const rows = data.solves.map((s, i) =>
    `<tr><td class="ch-num">${String(i + 1).padStart(2, "0")}</td><td class="ch-name">${s.challenge}</td><td><span class="ch-badge cat-${s.category}">${s.category}</span></td><td class="ch-pts">${s.points}</td><td class="ch-mono">${timeAgo(s.solved_at)}</td></tr>`
  ).join("");
  return `<div class="ch-table-wrap"><table><thead><tr><th>#</th><th>Challenge</th><th>Category</th><th>Points</th><th>Solved</th></tr></thead><tbody>${rows}</tbody></table></div>`;
}

function memberHeader(data: MemberData, backLabel: string, backBtnId: string): string {
  return `<div class="tm-detail-header">
    <button id="${backBtnId}" class="tm-back-btn">${icon("arrow-left")} ${backLabel}</button>
    <div class="tm-avatar tm-avatar-lg">${data.username[0].toUpperCase()}</div>
    <div>
      <div class="tm-detail-name">${data.username}</div>
      <div class="tm-detail-meta">${data.score} pts · ${data.solves.length} solved</div>
    </div>
  </div>`;
}

function showMemberProfile(data: MemberData) {
  const ws = document.getElementById("tm-workspace")!;
  const det = document.getElementById("tm-member-detail")!;
  det.innerHTML = memberHeader(data, "Back", "tm-back-btn") + memberSolvesHTML(data);
  ws.style.display = "none";
  det.style.display = "";
  document.getElementById("tm-back-btn")!.addEventListener("click", () => {
    det.style.display = "none";
    ws.style.display = "";
  });
}

document.querySelectorAll<HTMLTableRowElement>(".tm-member-row").forEach((row) => {
  const id = row.dataset.id;
  if (!id) return;
  row.style.cursor = "pointer";
  row.addEventListener("click", async () => {
    const res = await fetch(`/api/teams/members/${id}`);
    if (res.ok) showMemberProfile(await res.json());
  });
});

async function inspectTeam(team: any, rank: number) {
  const lb = document.getElementById("tab-leaderboard")!;
  const sorted = [...team.members].sort((a: any, b: any) => b.score - a.score);
  lb.innerHTML = `
    <div class="tm-detail-header">
      <button id="lb-back-btn" class="tm-back-btn">${icon("arrow-left")} Back</button>
      <div class="tm-avatar tm-avatar-lg">${team.name[0].toUpperCase()}</div>
      <div>
        <div class="tm-detail-name">${team.name}</div>
        <div class="tm-detail-meta">${team.score} pts · Rank #${rank}</div>
      </div>
    </div>
    <div class="ch-table-wrap" style="max-height:280px;flex:none;">
      <table>
        <thead><tr><th>#</th><th>Member</th><th>Score</th><th>Solves</th></tr></thead>
        <tbody>${sorted.map((m: any, i: number) => `
          <tr class="tm-member-row" data-id="${m.id}" style="cursor:pointer">
            <td class="ch-num">${String(i + 1).padStart(2, "0")}</td>
            <td><div class="tm-member-cell"><div class="tm-avatar">${m.username[0].toUpperCase()}</div><span class="ch-name">${m.username}</span></div></td>
            <td class="ch-pts">${m.score}</td>
            <td class="ch-mono">${m.solves ?? 0}</td>
          </tr>`).join("")}
        </tbody>
      </table>
    </div>`;

  document.getElementById("lb-back-btn")!.addEventListener("click", () => bindLeaderboardRows());

  lb.querySelectorAll<HTMLTableRowElement>(".tm-member-row").forEach((row) => {
    row.addEventListener("click", async () => {
      const res = await fetch(`/api/teams/members/${row.dataset.id}`);
      if (res.ok) inspectMember(await res.json(), team, rank);
    });
  });

  const actRes = await fetch(`/api/teams/${team.id}/activity`);
  if (actRes.ok && lb.contains(document.getElementById("lb-back-btn"))) {
    const activity: any[] = await actRes.json();
    if (activity.length > 0) {
      const entries = activity.map(e => `
        <div class="tm-feed-entry">
          <div class="tm-avatar tm-avatar-sm">${e.username[0].toUpperCase()}</div>
          <div class="tm-feed-body">
            <div class="tm-feed-line">
              <span class="tm-feed-user">${e.username}</span>
              <span class="tm-feed-verb">solved</span>
              <span class="tm-feed-chall cat-${e.category}">${e.challenge}</span>
              <span class="tm-feed-pts">+${e.points}</span>
            </div>
            <div class="tm-feed-time">${timeAgo(e.solved_at)}</div>
          </div>
        </div>`).join("");
      lb.insertAdjacentHTML("beforeend",
        `<div class="tm-activity-section"><div class="tm-activity-label">Recent Activity</div><div class="tm-activity-scroll">${entries}</div></div>`);
    }
  }
}

function inspectMember(data: MemberData, team: any, rank: number) {
  const lb = document.getElementById("tab-leaderboard")!;
  lb.innerHTML = memberHeader(data, "Back to team", "lb-back-btn") + memberSolvesHTML(data);
  document.getElementById("lb-back-btn")!.addEventListener("click", () => inspectTeam(team, rank));
}

function bindLeaderboardRows() {
  const lb = document.getElementById("tab-leaderboard")!;
  lb.innerHTML = (window as any).__lbHTML;
  lb.querySelectorAll<HTMLTableRowElement>(".tm-row").forEach((row) => {
    row.addEventListener("click", () => {
      if (row.classList.contains("tm-mine")) {
        document.querySelector<HTMLButtonElement>('[data-tab="team"]')!.click();
        return;
      }
      const team = JSON.parse(row.dataset.team!);
      const rank = parseInt(row.dataset.rank!);
      inspectTeam(team, rank);
    });
  });
}

// Save original leaderboard HTML and bind on load
(window as any).__lbHTML = document.getElementById("tab-leaderboard")!.innerHTML;
bindLeaderboardRows();

const memberId = (window as any).__inspectMemberId;
if (memberId) {
  const teamsData = (window as any).__teamsData as any[];
  const team = teamsData?.find((t: any) => t.members.some((m: any) => m.id === memberId));
  if (team) {
    const rank = teamsData.indexOf(team) + 1;
    document.querySelectorAll(".tm-tab").forEach((b) => b.classList.remove("active"));
    document.querySelector<HTMLButtonElement>('[data-tab="leaderboard"]')!.classList.add("active");
    document.getElementById("tab-team")!.style.display = "none";
    document.getElementById("tab-leaderboard")!.style.display = "";
    fetch(`/api/teams/members/${memberId}`).then(r => r.json()).then(data => inspectMember(data, team, rank));
  }
}

document.getElementById("tm-copy-btn")?.addEventListener("click", () => {
  const code = document.getElementById("tm-invite-code")?.textContent ?? "";
  navigator.clipboard.writeText(code);
  const btn = document.getElementById("tm-copy-btn") as HTMLButtonElement;
  btn.classList.add("copied");
  setTimeout(() => btn.classList.remove("copied"), 1500);
});
