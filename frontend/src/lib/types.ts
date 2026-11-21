export type User = {
  id: string;
  username: string;
  score: number;
  role: "user" | "maintainer" | "admin";
  rank: number;
};

export type Member = {
  id: string;
  username: string;
  score: number;
  solves?: number;
};

export type Team = {
  id: string;
  name: string;
  captain_id: string | null;
  banned: boolean;
  hidden: boolean;
  member_count: number;
  score: number;
  rank: number;
};

export type MyTeam = Team & {
  members: Member[];
  invite_code?: string;
};

export type PublicSettings = {
  ctf_name: string;
  ctf_start: string;
  ctf_end: string;
  registration: boolean;
  max_team_size: number;
  rules: string;
};

export type Challenge = {
  id: string;
  title: string;
  description: string | null;
  author: string | null;
  points: number;
  category: string;
  difficulty: string;
  solves: number;
  connection_info: string | null;
  files: string[];
  docker_image: string | null;
  docker_port: number | null;
  is_visible: boolean;
};

export type SubmitResult = {
  correct: boolean;
  challenge_id: string | null;
  points: number;
};

export type Solve = {
  uid: string;
  username: string;
  team: string | null;
  solved_at: string | null;
};

export type ActivityEntry = {
  username: string;
  challenge: string;
  category: string;
  points: number;
  solved_at: string | null;
};

export type MemberDetail = {
  id: string;
  username: string;
  score: number;
  solves: { challenge: string; category: string; points: number; solved_at: string | null }[];
};

export type Instance = {
  id: string;
  name: string;
  image: string;
  status: string;
  provider: string;
  created_at: string;
  host_ip: string | null;
  host_port: string | null;
};
