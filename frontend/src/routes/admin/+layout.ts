import { requireUser } from "@library/load";
import type { LayoutLoad } from "./$types";

export const load: LayoutLoad = requireUser;
