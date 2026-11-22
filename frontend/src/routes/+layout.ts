import { ApiError, me, settings as loadSettings } from "@library/api";
import type { LayoutLoad } from "./$types";

export const ssr = false;

export const load: LayoutLoad = async () => {
  const settings = loadSettings().catch(() => null);
  try {
    return { user: await me(), signedOutBy: null, settings: await settings };
  } catch (error) {
    if (error instanceof ApiError && (error.status === 401 || error.status === 403)) {
      return { user: null, signedOutBy: String(error.status), settings: await settings };
    }
    throw error;
  }
};
