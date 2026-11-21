import * as api from "@library/api";
import { required } from "@library/load";
import type { PageLoad } from "./$types";

export const load: PageLoad = async ({ params }) => ({
  instance: (await required(api.listInstances())).find((i) => i.id === params.id) ?? null,
});
