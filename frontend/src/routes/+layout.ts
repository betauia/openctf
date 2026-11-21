import { ApiError, me } from "@library/api";
import type { LayoutLoad } from "./$types";

export const ssr = false;

export const load: LayoutLoad = async () => {
  try {
    return { user: await me(), signedOutBy: null };
  } catch (error) {
    if (error instanceof ApiError && (error.status === 401 || error.status === 403)) {
      return { user: null, signedOutBy: String(error.status) };
    }
    throw error;
  }
};
