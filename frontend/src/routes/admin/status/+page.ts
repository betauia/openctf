import * as api from "#library/api.js";
import { tolerated } from "#library/load.js";
import type { PageLoad } from "./$types";

export const load: PageLoad = async () => ({
  instances: await tolerated(api.listInstances(), []),
});
