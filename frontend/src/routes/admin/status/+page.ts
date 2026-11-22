import * as api from "@library/api";
import { tolerated } from "@library/load";
import type { PageLoad } from "./$types";

export const load: PageLoad = async () => ({
  instances: await tolerated(api.listInstances(), []),
});
