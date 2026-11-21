import * as api from "@library/api";
import { required } from "@library/load";
import type { PageLoad } from "./$types";

export const load: PageLoad = async () => ({
  instances: await required(api.listInstances()),
});
