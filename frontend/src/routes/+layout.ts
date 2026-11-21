import { ApiError, me, settings } from "@library/api";
import type { LayoutLoad } from "./$types";

export const ssr = false;

export const load: LayoutLoad = async () => {
  const theme = settings()
    .then((s) => s.theme)
    .catch(() => null);
  try {
    return { user: await me(), signedOutBy: null, theme: await theme };
  } catch (error) {
    if (error instanceof ApiError && (error.status === 401 || error.status === 403)) {
      return { user: null, signedOutBy: String(error.status), theme: await theme };
    }
    throw error;
  }
};
