import { chromium, type FullConfig } from "@playwright/test";

export default async function login(config: FullConfig) {
  const { OCTF_USER, OCTF_PASS } = process.env;
  if (!OCTF_USER || !OCTF_PASS) throw new Error("Set OCTF_USER and OCTF_PASS to a user that is in a team");

  const browser = await chromium.launch();
  const page = await browser.newPage({ baseURL: config.projects[0].use.baseURL });
  await page.goto("/login");
  await page.fill("#username", OCTF_USER);
  await page.fill("#password", OCTF_PASS);
  await page.click("button[type=submit]");
  await page.waitForURL((url) => !url.pathname.startsWith("/login"));
  await page.context().storageState({ path: "tests/.auth.json" });
  await browser.close();
}
