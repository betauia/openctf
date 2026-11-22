import { expect, test, type Page } from "@playwright/test";

const screens = [
  { name: "phone", width: 390, height: 844 },
  { name: "half-1440p", width: 1430, height: 1125 },
  { name: "tablet", width: 768, height: 1024 },
  { name: "laptop", width: 1366, height: 768 },
  { name: "1080p", width: 1920, height: 1080 },
  { name: "1440p", width: 2560, height: 1440 },
  { name: "ultrawide", width: 3440, height: 1440 },
  { name: "super-ultrawide", width: 5120, height: 1440 },
  { name: "4k", width: 3840, height: 2160 },
];

const pages = [
  "/challenges",
  "/teams",
  "/teams/scout",
  "/scoreboard",
  "/rules",
  "/admin/status",
  "/admin/config",
  "/login",
  "/register",
  "/does-not-exist",
];

const MIN_TEXT_PX = 8;

async function layoutProblems(page: Page) {
  return page.evaluate((minText) => {
    const vw = document.documentElement.clientWidth;
    const problems: string[] = [];
    const insideScroller = (el: Element) => {
      for (let a = el.parentElement; a && a !== document.body; a = a.parentElement) {
        const o = getComputedStyle(a).overflowX;
        if (o === "auto" || o === "scroll" || o === "hidden") return true;
      }
      return false;
    };
    const label = (el: Element) =>
      `<${el.tagName.toLowerCase()}> "${((el as HTMLElement).innerText ?? "").trim().slice(0, 30)}"`;

    if (document.documentElement.scrollWidth > vw) {
      problems.push(`page scrolls sideways by ${document.documentElement.scrollWidth - vw}px`);
    }
    for (const el of document.querySelectorAll("body *")) {
      const style = getComputedStyle(el);
      if (style.display === "none" || style.visibility === "hidden") continue;
      const box = el.getBoundingClientRect();
      if (!box.width || !box.height) continue;
      if (box.right > vw + 1 && style.position !== "fixed" && !insideScroller(el)) {
        problems.push(`${label(el)} sticks out ${Math.round(box.right - vw)}px past the screen`);
      }
      const hasText = [...el.childNodes].some((n) => n.nodeType === Node.TEXT_NODE && n.textContent?.trim());
      if (hasText && parseFloat(style.fontSize) < minText) {
        problems.push(`${label(el)} text is ${style.fontSize}`);
      }
    }
    return [...new Set(problems)];
  }, MIN_TEXT_PX);
}

function trackErrors(page: Page) {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  return errors;
}

test.beforeEach(async ({ page }) => {
  await page.route("**/api/live", (route) => route.abort());
});

for (const path of pages) {
  for (const screen of screens) {
    test(`${path} fits on ${screen.name} (${screen.width}x${screen.height})`, async ({ page }) => {
      const errors = trackErrors(page);
      await page.setViewportSize({ width: screen.width, height: screen.height });
      await page.goto(path);
      await page.waitForLoadState("networkidle");
      expect(await layoutProblems(page)).toEqual([]);
      expect(errors).toEqual([]);
    });
  }

  test(`${path} survives live resizing`, async ({ page }) => {
    const errors = trackErrors(page);
    await page.setViewportSize({ width: 1920, height: 1080 });
    await page.goto(path);
    await page.waitForLoadState("networkidle");
    for (const screen of [...screens, ...[...screens].reverse()]) {
      await page.setViewportSize({ width: screen.width, height: screen.height });
      await page.waitForTimeout(150);
      const problems = await layoutProblems(page);
      expect(problems, `after resizing to ${screen.name}`).toEqual([]);
    }
    expect(errors).toEqual([]);
  });
}
