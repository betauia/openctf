<script lang="ts">
  import LeaderboardTab from "@components/teams/LeaderboardTab.svelte";
  import { pad2 } from "@library/utils";
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  const CSS_COLORS = [
    "--accent-color",
    "--accent-secondary",
    "--cat-misc",
    "--cat-rev",
    "--cat-crypto",
    "--cat-beginner",
    "--cat-osint",
    "--cat-hardware",
    "--cat-forensics",
    "--cat-stego",
  ];

  function resolveColors(): string[] {
    const style = getComputedStyle(document.documentElement);
    return CSS_COLORS.map((v) => style.getPropertyValue(v).trim() || "#888");
  }

  const COLORS = resolveColors();

  let series = $derived(
    data.series.map((s, i) => ({
      id: s.team_id,
      name: s.team_name,
      color: COLORS[i % COLORS.length],
      points: s.points.map((p) => [new Date(p.at).getTime(), p.score] as [number, number]),
    })),
  );

  let allPoints = $derived(series.flatMap((s) => s.points));
  let xMin = $derived(allPoints.length ? Math.min(...allPoints.map((p) => p[0])) : 0);
  let xMax = $derived(allPoints.length ? Math.max(...allPoints.map((p) => p[0])) : 1);
  let yMaxPadded = $derived(allPoints.length ? Math.ceil(Math.max(...allPoints.map((p) => p[1])) * 1.1) : 1);

  const readRem = () => parseFloat(getComputedStyle(document.documentElement).fontSize) || 16;
  let remPx = $state(readRem());
  let chartWidth = $state(900);
  let W = $derived(Math.max(280, chartWidth));
  let H = $derived((W < 37.5 * remPx ? 15 : 20) * remPx);
  let PAD = $derived({ top: 1.25 * remPx, right: remPx, bottom: 2 * remPx, left: 2.75 * remPx });
  let plotW = $derived(W - PAD.left - PAD.right);
  let plotH = $derived(H - PAD.top - PAD.bottom);

  function sx(t: number) {
    if (xMax === xMin) return PAD.left + plotW / 2;
    return PAD.left + ((t - xMin) / (xMax - xMin)) * plotW;
  }
  function sy(v: number) {
    if (yMaxPadded === 0) return PAD.top + plotH;
    return PAD.top + plotH - (v / yMaxPadded) * plotH;
  }

  function stepPath(pts: [number, number][]): string {
    if (pts.length === 0) return "";
    let d = `M${sx(pts[0][0]).toFixed(1)},${sy(pts[0][1]).toFixed(1)}`;
    for (let i = 1; i < pts.length; i++) {
      d += `H${sx(pts[i][0]).toFixed(1)}`;
      d += `V${sy(pts[i][1]).toFixed(1)}`;
    }
    d += `H${sx(xMax).toFixed(1)}`;
    return d;
  }

  function fmtTime(t: number): string {
    const d = new Date(t);
    return `${d.getMonth() + 1}/${d.getDate()} ${pad2(d.getHours())}:${pad2(d.getMinutes())}`;
  }

  let yTicks = $derived.by(() => {
    if (yMaxPadded === 0) return [0];
    const count = 5;
    const raw = yMaxPadded / count;
    const mag = Math.pow(10, Math.floor(Math.log10(raw)));
    const step = Math.ceil(raw / mag) * mag;
    const ticks = [];
    for (let v = 0; v <= yMaxPadded; v += step) ticks.push(v);
    return ticks;
  });

  let xTicks = $derived.by(() => {
    if (xMax === xMin) return [xMin];
    const count = Math.max(2, Math.min(8, Math.floor(plotW / (7.5 * remPx))));
    const step = (xMax - xMin) / count;
    return Array.from({ length: count + 1 }, (_, i) => xMin + step * i);
  });

  let hovered = $state<string | null>(null);
  let mouseX = $state<number | null>(null);
  let chartEl: SVGSVGElement | undefined = $state();

  function onChartMove(e: MouseEvent) {
    if (!chartEl) return;
    const rect = chartEl.getBoundingClientRect();
    const svgX = ((e.clientX - rect.left) / rect.width) * W;
    if (svgX < PAD.left || svgX > W - PAD.right) {
      mouseX = null;
      return;
    }
    mouseX = svgX;
  }

  let crosshairTime = $derived(mouseX !== null ? xMin + ((mouseX - PAD.left) / plotW) * (xMax - xMin) : null);

  function scoreAt(pts: [number, number][], t: number): number {
    let score = 0;
    for (const p of pts) {
      if (p[0] <= t) score = p[1];
      else break;
    }
    return score;
  }

  let tooltipData = $derived.by(() => {
    const t = crosshairTime;
    if (t === null) return null;
    return series
      .map((s) => ({ name: s.name, color: s.color, score: scoreAt(s.points, t) }))
      .filter((d) => d.score > 0)
      .sort((a, b) => b.score - a.score);
  });
</script>

<svelte:window onresize={() => (remPx = readRem())} />

<svelte:head><title>Scoreboard - OpenCTF</title></svelte:head>

<div class="flex flex-col h-[calc(100vh-3rem)] mt-12 bg-raised overflow-y-auto">
  <div class="w-full max-w-[75rem] mx-auto">
    {#if data.teams.length === 0}
      <div class="flex-1 flex items-center justify-center text-[0.8125rem] text-text/30">No teams yet</div>
    {:else}
      {#if series.length > 0}
        <div class="mx-6 mt-6 rounded-lg bg-elevated border border-text/6 p-5">
          <div class="flex items-center justify-between mb-4">
            <div class="text-[0.8125rem] font-medium text-text/80">Score over Time</div>
            <div class="flex flex-wrap gap-x-4 gap-y-1">
              {#each series as s (s.id)}
                <button
                  class="flex items-center gap-1.5 text-[0.6875rem] bg-transparent border-none cursor-pointer p-0
                  transition-opacity duration-150"
                  style="color: {s.color}; opacity: {hovered !== null && hovered !== s.id ? 0.25 : 1}"
                  onmouseenter={() => (hovered = s.id)}
                  onmouseleave={() => (hovered = null)}
                >
                  <span class="w-3 h-[0.125rem] inline-block rounded-full" style="background: {s.color}"></span>
                  {s.name}
                </button>
              {/each}
            </div>
          </div>

          <div class="w-full relative" bind:clientWidth={chartWidth}>
            <svg
              role="img"
              aria-label="Team scores over time"
              bind:this={chartEl}
              width={W}
              height={H}
              viewBox="0 0 {W} {H}"
              class="w-full block"
              onmousemove={onChartMove}
              onmouseleave={() => (mouseX = null)}
            >
              <defs>
                {#each series as s (s.id)}
                  <linearGradient id="grad-{s.id}" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stop-color={s.color} stop-opacity="0.15" />
                    <stop offset="100%" stop-color={s.color} stop-opacity="0" />
                  </linearGradient>
                {/each}
              </defs>

              {#each yTicks as tick}
                <line
                  x1={PAD.left}
                  y1={sy(tick)}
                  x2={W - PAD.right}
                  y2={sy(tick)}
                  stroke="rgba(255,255,255,0.05)"
                  stroke-width="1"
                />
                <text
                  x={PAD.left - 0.625 * remPx}
                  y={sy(tick) + 0.25 * remPx}
                  text-anchor="end"
                  fill="rgba(255,255,255,0.3)"
                  font-size="0.625rem"
                  font-family="Arial, sans-serif"
                >
                  {tick}
                </text>
              {/each}

              {#each xTicks as tick, i}
                <line
                  x1={sx(tick)}
                  y1={PAD.top}
                  x2={sx(tick)}
                  y2={PAD.top + plotH}
                  stroke="rgba(255,255,255,0.03)"
                  stroke-width="1"
                />
                <text
                  x={sx(tick)}
                  y={H - 0.5 * remPx}
                  text-anchor={i === 0 ? "start" : i === xTicks.length - 1 ? "end" : "middle"}
                  fill="rgba(255,255,255,0.3)"
                  font-size="0.5625rem"
                  font-family="Arial, sans-serif"
                >
                  {fmtTime(tick)}
                </text>
              {/each}

              {#each series as s (s.id)}
                {@const pts = s.points}
                {@const areaPath = stepPath(pts) + `V${PAD.top + plotH}H${sx(pts[0][0]).toFixed(1)}Z`}
                <path
                  d={areaPath}
                  fill="url(#grad-{s.id})"
                  opacity={hovered === null || hovered === s.id ? 1 : 0}
                  style="transition: opacity 0.15s"
                />
                <path
                  d={stepPath(pts)}
                  fill="none"
                  stroke={s.color}
                  stroke-width={0.125 * remPx}
                  opacity={hovered === null || hovered === s.id ? 1 : 0.15}
                  style="transition: opacity 0.15s"
                />
                {#each pts as p}
                  <circle
                    cx={sx(p[0])}
                    cy={sy(p[1])}
                    r={0.1875 * remPx}
                    fill={s.color}
                    opacity={hovered === null || hovered === s.id ? 1 : 0.15}
                    style="transition: opacity 0.15s"
                  />
                {/each}
              {/each}

              {#if mouseX !== null}
                <line
                  x1={mouseX}
                  y1={PAD.top}
                  x2={mouseX}
                  y2={PAD.top + plotH}
                  stroke="rgba(255,255,255,0.2)"
                  stroke-width="1"
                  stroke-dasharray="3,3"
                />
              {/if}
            </svg>

            {#if mouseX !== null && tooltipData && tooltipData.length > 0}
              {@const tooltipLeft = (mouseX / W) * 100}
              {@const flipLeft = tooltipLeft > 70}
              <div
                class="absolute top-2 pointer-events-none bg-background border border-text/10
                rounded-md px-3 py-2 text-[0.6875rem] min-w-28 z-10"
                style="left: {flipLeft ? tooltipLeft - 1 : tooltipLeft + 1}%;
                transform: translateX({flipLeft ? '-100%' : '0'});"
              >
                {#each tooltipData as d}
                  <div class="flex items-center justify-between gap-4 py-0.5">
                    <div class="flex items-center gap-1.5">
                      <span class="w-2 h-2 rounded-full inline-block" style="background: {d.color}"></span>
                      <span style="color: {d.color}">{d.name}</span>
                    </div>
                    <span class="text-text font-bold">{d.score}</span>
                  </div>
                {/each}
              </div>
            {/if}
          </div>
        </div>
      {/if}

      <LeaderboardTab teams={data.teams} myTeam={data.myTeam} />
    {/if}
  </div>
</div>
