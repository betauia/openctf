<script lang="ts">
  import type { ScoreEvent } from "@library/api";
  import type { Team } from "@library/types";
  import { initial, pad2 } from "@library/utils";
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  let teams = $state(data.teams);
  let myTeam = $derived(data.myTeam);
  let events = $state(data.events);

  $effect(() => {
    teams = data.teams;
    events = data.events;
  });

  const CSS_COLORS = [
    "--accent-color",
    "--cat-web",
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

  let COLORS = $state(resolveColors());

  $effect(() => {
    COLORS = resolveColors();
  });

  type Series = { id: string; name: string; color: string; points: [number, number][] };

  let series = $derived.by(() => {
    if (events.length === 0) return [];

    const cumulative = new Map<string, { name: string; score: number; points: [number, number][] }>();
    const topIds = teams.slice(0, 10).map((t) => t.id);

    for (const e of events) {
      if (!topIds.includes(e.team_id)) continue;
      let entry = cumulative.get(e.team_id);
      if (!entry) {
        entry = { name: e.team_name, score: 0, points: [] };
        cumulative.set(e.team_id, entry);
      }
      entry.score += e.points;
      entry.points.push([new Date(e.solved_at).getTime(), entry.score]);
    }

    return topIds
      .filter((id) => cumulative.has(id))
      .map((id, i) => {
        const e = cumulative.get(id)!;
        return { id, name: e.name, color: COLORS[i % COLORS.length], points: e.points } satisfies Series;
      });
  });

  let xMin = $derived(series.length ? Math.min(...series.flatMap((s) => s.points.map((p) => p[0]))) : 0);
  let xMax = $derived(series.length ? Math.max(...series.flatMap((s) => s.points.map((p) => p[0]))) : 1);
  let yMax = $derived(series.length ? Math.max(...series.flatMap((s) => s.points.map((p) => p[1]))) : 1);
  let yMaxPadded = $derived(Math.ceil(yMax * 1.1));

  const W = 900;
  const H = 340;
  const PAD = { top: 24, right: 24, bottom: 36, left: 56 };
  const plotW = W - PAD.left - PAD.right;
  const plotH = H - PAD.top - PAD.bottom;

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
    const h = String(d.getHours()).padStart(2, "0");
    const m = String(d.getMinutes()).padStart(2, "0");
    return `${d.getMonth() + 1}/${d.getDate()} ${h}:${m}`;
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
    const count = 6;
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

  function onChartLeave() {
    mouseX = null;
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
    if (crosshairTime === null) return null;
    return series
      .map((s) => ({ name: s.name, color: s.color, score: scoreAt(s.points, crosshairTime!) }))
      .filter((d) => d.score > 0)
      .sort((a, b) => b.score - a.score);
  });
</script>

<svelte:head><title>Scoreboard - OpenCTF</title></svelte:head>

<div class="flex flex-col h-[calc(100vh-48px)] mt-12 bg-raised overflow-y-auto">
  {#if teams.length === 0}
    <div class="flex-1 flex items-center justify-center text-[13px] text-text/30">No teams yet</div>
  {:else}
    {#if series.length > 0}
      <div class="mx-5 mt-6 mb-4 rounded-lg bg-elevated border border-text/6 p-5">
        <div class="flex items-center justify-between mb-4">
          <div class="text-[13px] font-medium text-text/80">Score over Time</div>
          <div class="flex flex-wrap gap-x-4 gap-y-1">
            {#each series as s (s.id)}
              <button
                class="flex items-center gap-1.5 text-[11px] bg-transparent border-none cursor-pointer p-0
                  transition-opacity duration-150"
                style="color: {s.color}; opacity: {hovered !== null && hovered !== s.id ? 0.25 : 1}"
                onmouseenter={() => (hovered = s.id)}
                onmouseleave={() => (hovered = null)}
              >
                <span class="w-3 h-[2px] inline-block rounded-full" style="background: {s.color}"></span>
                {s.name}
              </button>
            {/each}
          </div>
        </div>

        <div class="w-full overflow-x-auto relative">
          <svg
            bind:this={chartEl}
            viewBox="0 0 {W} {H}"
            class="w-full"
            style="min-width: 500px;"
            onmousemove={onChartMove}
            onmouseleave={onChartLeave}
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
                x={PAD.left - 10}
                y={sy(tick) + 4}
                text-anchor="end"
                fill="rgba(255,255,255,0.3)"
                font-size="10"
                font-family="Arial, sans-serif"
              >
                {tick}
              </text>
            {/each}

            {#each xTicks as tick}
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
                y={H - 8}
                text-anchor="middle"
                fill="rgba(255,255,255,0.3)"
                font-size="9"
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
                stroke-width="2"
                opacity={hovered === null || hovered === s.id ? 1 : 0.15}
                style="transition: opacity 0.15s"
              />
              {#each pts as p}
                <circle
                  cx={sx(p[0])}
                  cy={sy(p[1])}
                  r="3"
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
                rounded-md px-3 py-2 text-[11px] min-w-28 z-10"
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

    <div class="px-4 pb-6">
      <table class="w-full border-separate border-spacing-x-0 border-spacing-y-0.5">
        <thead>
          <tr>
            {#each ["#", "Team", "Members", "Score"] as header, i (header)}
              <th class="th {i >= 2 ? 'text-center' : 'text-left'}">
                {header}
              </th>
            {/each}
          </tr>
        </thead>
        <tbody>
          {#each teams as team, i (team.id)}
            {@const mine = team.id === myTeam?.id}
            <tr class="trow">
              <td class="td text-[11px] w-9 {mine ? 'text-accent' : 'text-text/18'}">
                {pad2(i + 1)}
              </td>
              <td class="td text-[13px] font-medium {mine ? 'text-accent' : 'text-muted-text'}">
                <div class="flex items-center gap-2.5">
                  <div class="avatar w-7 h-7 text-[12px]">
                    {initial(team.name)}
                  </div>
                  {team.name}
                </div>
              </td>
              <td class="td text-[12px] text-text/45 text-center">{team.member_count}</td>
              <td class="td text-[13px] font-bold text-pts text-center">{team.score ?? "—"}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>
