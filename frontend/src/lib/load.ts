import { error } from "@sveltejs/kit";
import { ApiError } from "./api";

/**
 * Turns a failed API call into a SvelteKit error that keeps the backend's status
 * and message, so +error.svelte can show what actually went wrong instead of a
 * blank 500. Use it for data a route cannot render without.
 */
export async function required<T>(call: Promise<T>): Promise<T> {
  try {
    return await call;
  } catch (cause) {
    if (cause instanceof ApiError) error(cause.status, cause.message);
    throw cause;
  }
}

/**
 * For endpoints where 404 is a real answer rather than a failure — GET
 * /api/teams/mine returns it when the user is in no team. Other statuses still
 * surface as errors.
 */
export async function optional<T>(call: Promise<T>): Promise<T | null> {
  try {
    return await call;
  } catch (cause) {
    if (cause instanceof ApiError && cause.status === 404) return null;
    return required(Promise.reject(cause));
  }
}

/**
 * For supplementary data a route can render without — the activity feed, a member
 * profile. Returns the failure as a message instead of erroring the whole page,
 * so the UI can say what is missing rather than showing a silent blank.
 */
export async function tolerated<T>(call: Promise<T>, fallback: T): Promise<{ value: T; error: string | null }> {
  try {
    return { value: await call, error: null };
  } catch (cause) {
    return { value: fallback, error: cause instanceof Error ? cause.message : String(cause) };
  }
}
