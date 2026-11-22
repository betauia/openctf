import * as api from "@library/api";
import { tolerated } from "@library/load";
import type { PageLoad } from "./$types";

export const load: PageLoad = async ({ params }) => {
  const instances = await tolerated(api.listInstances(), []);
  return {
    instance: instances.value.find((i) => i.id === params.id) ?? null,
    error: instances.error,
  };
};
