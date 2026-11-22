import * as api from "@library/api";
import { required } from "@library/load";
import type { PageLoad } from "./$types";

export const load: PageLoad = async ({ parent }) => ({
  settings: (await parent()).settings ?? (await required(api.settings())),
});
