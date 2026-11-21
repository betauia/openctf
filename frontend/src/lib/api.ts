import type {
  ActivityEntry,
  Challenge,
  ChallengeFile,
  Comparison,
  Instance,
  MemberDetail,
  MyTeam,
  Progress,
  PublicSettings,
  ScoreSeries,
  Solve,
  SubmitResult,
  Team,
  User,
} from "./types";

export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

function getToken(key: string): string | null {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

function setToken(key: string, value: string | null) {
  try {
    if (value) localStorage.setItem(key, value);
    else localStorage.removeItem(key);
  } catch {}
}

function storeTokens(response: Response) {
  const s = response.headers.get("x-session");
  const r = response.headers.get("x-refresh");
  if (s) setToken("session", s);
  if (r) setToken("refresh", r);
}

async function send(path: string, method: string, body: unknown) {
  const headers: Record<string, string> = {};
  if (body !== undefined) headers["Content-Type"] = "application/json";
  const s = getToken("session");
  const r = getToken("refresh");
  if (s) headers["x-session"] = s;
  if (r) headers["x-refresh"] = r;

  return fetch(`/api${path}`, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  });
}

async function authed(path: string, method: string, body: unknown) {
  let response = await send(path, method, body);
  storeTokens(response);

  if (response.status === 401 && path !== "/auth/refresh" && path !== "/auth/login") {
    const refreshed = await send("/auth/refresh", "POST", undefined);
    storeTokens(refreshed);
    if (refreshed.ok) {
      response = await send(path, method, body);
      storeTokens(response);
    }
  }

  if (!response.ok) {
    const body = await response.json().catch(() => ({}));
    throw new ApiError(
      response.status,
      body.detail ?? body.error ?? `${method} /api${path} failed with ${response.status}`,
    );
  }
  return response;
}

async function request<T>(
  path: string,
  { method = "GET", body }: { method?: string; body?: unknown } = {},
): Promise<T> {
  const response = await authed(path, method, body);
  return response.status === 204 ? (undefined as T) : response.json();
}

type Ok = { ok: boolean };

export const login = (body: { username: string; password: string }) =>
  request<{ id: string; username: string }>("/auth/login", { method: "POST", body });

export const register = (body: { username: string; email: string; password: string }) =>
  request<{ id: string; username: string }>("/auth/register", { method: "POST", body });

export const logout = () => {
  setToken("session", null);
  setToken("refresh", null);
  return request<Ok>("/auth/logout", { method: "POST" });
};

export const me = () => request<User>("/auth/me");

export const mySolves = () => request<string[]>("/auth/solves");

export const listTeams = () => request<Team[]>("/teams?limit=100");

export const myTeam = () => request<MyTeam>("/teams/mine");

export const createTeam = (name: string) => request<{ id: string }>("/teams", { method: "POST", body: { name } });

export const joinTeam = (invite_code: string) => request<Ok>("/teams/join", { method: "POST", body: { invite_code } });

export const leaveTeam = () => request<Ok>("/teams/mine/leave", { method: "POST" });

export const updateTeam = (body: { name?: string; regenerate_invite?: boolean }) =>
  request<Ok & { invite_code?: string }>("/teams/mine", { method: "PATCH", body });

export const transferCaptain = (user_id: string) =>
  request<Ok>("/teams/mine/captain", { method: "PUT", body: { user_id } });

export const kickMember = (uid: string) => request<Ok>(`/teams/mine/members/${uid}`, { method: "DELETE" });

export const memberDetail = (id: string) => request<MemberDetail>(`/users/${id}`);

export const teamActivity = (teamId: string) => request<ActivityEntry[]>(`/teams/${teamId}/activity`);

export const teamProgress = (teamId: string) => request<Progress>(`/teams/${teamId}/progress`);

export const compareTeam = (teamId: string) => request<Comparison>(`/teams/${teamId}/compare`);

export const settings = () => request<PublicSettings>("/settings");

export const updateSettings = (body: Partial<PublicSettings>) =>
  request<PublicSettings>("/admin/settings", { method: "PUT", body });

export const scoreboard = () => request<ScoreSeries[]>("/scoreboard");

export const listChallenges = () => request<Challenge[]>("/challenges");

export const challengeSolves = (id: string) => request<Solve[]>(`/challenges/${id}/solves`);

export const submitFlag = (flag: string, challenge_id?: string) =>
  request<SubmitResult>("/challenges/submit", { method: "POST", body: { flag, challenge_id } });

export const challengeFiles = (challengeId: string) => request<ChallengeFile[]>(`/challenges/${challengeId}/files`);

export async function downloadFile(challengeId: string, file: ChallengeFile) {
  const response = await authed(`/challenges/${challengeId}/files/${file.id}`, "GET", undefined);
  const url = URL.createObjectURL(await response.blob());
  const link = Object.assign(document.createElement("a"), { href: url, download: file.filename });
  link.click();
  URL.revokeObjectURL(url);
}

export const listInstances = () => request<Instance[]>("/instances");

export const createInstance = (challenge_id: string) =>
  request<Instance>("/instances", { method: "POST", body: { challenge_id } });

export const stopInstance = (id: string) => request<void>(`/instances/${id}`, { method: "DELETE" });
