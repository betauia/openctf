import * as api from "#library/api.js";
import { required } from "#library/load.js";
import type { PageLoad } from "./$types";

export const load: PageLoad = async ({ parent }) => ({
  settings: (await parent()).settings ?? (await required(api.settings())),
});
