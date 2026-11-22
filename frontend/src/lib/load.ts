import { error, redirect } from "@sveltejs/kit";
import { ApiError } from "./api";
import { loginFor } from "./redirect";
import type { User } from "./types";

export async function requireUser<T extends { user: User | null; signedOutBy: string | null }>({
  parent,
  url,
}: {
  parent: () => Promise<T>;
  url: URL;
}): Promise<T & { user: User }> {
  const data = await parent();
  if (!data.user) redirect(302, loginFor(url.pathname + url.search, data.signedOutBy ?? undefined));
  return data as T & { user: User };
}

export async function required<T>(call: Promise<T>): Promise<T> {
  try {
    return await call;
  } catch (cause) {
    if (cause instanceof ApiError) error(cause.status, cause.message);
    throw cause;
  }
}

export async function optional<T>(call: Promise<T>): Promise<T | null> {
  try {
    return await call;
  } catch (cause) {
    if (cause instanceof ApiError && cause.status === 404) return null;
    return required(Promise.reject(cause));
  }
}

export async function tolerated<T>(call: Promise<T>, fallback: T): Promise<{ value: T; error: string | null }> {
  try {
    return { value: await call, error: null };
  } catch (cause) {
    return { value: fallback, error: cause instanceof Error ? cause.message : String(cause) };
  }
}
