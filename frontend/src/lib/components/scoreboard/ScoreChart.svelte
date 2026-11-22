<script lang="ts" module>
  export type Series = {
    id: string;
    name: string;
    color: string;
    points: [number, number][];
  };
</script>

<script lang="ts">
  import { pad2 } from "@library/utils";

  let { series, start, end }: { series: Series[]; start: number | null; end: number } = $props();

  const readRem = () => parseFloat(getComputedStyle(document.documentElement).fontSize) || 16;
  let rem = $state(readRem());
  let width = $state(900);

  let points = $derived(series.flatMap((s) => s.points));
  let xMin = $derived(Math.min(start ?? end, ...points.map((p) => p[0])));
  let xMax = $derived(Math.max(end, ...points.map((p) => p[0])));
  let yMax = $derived(Math.ceil(Math.max(0, ...points.map((p) => p[1])) * 1.1));

  let W = $derived(Math.max(280, width));
  let H = $derived((W < 37.5 * rem ? 15 : 20) * rem);
  let pad = $derived({
    top: 1.25 * rem,
    right: rem,
    bottom: 2 * rem,
    left: 2.75 * rem,
  });
  let plotW = $derived(W - pad.left - pad.right);
  let plotH = $derived(H - pad.top - pad.bottom);
  let bottom = $derived(pad.top + plotH);

  const sx = (t: number) => pad.left + (xMax === xMin ? 0.5 : (t - xMin) / (xMax - xMin)) * plotW;
  const sy = (v: number) => bottom - (yMax === 0 ? 0 : v / yMax) * plotH;

  const stepPath = (pts: [number, number][]) =>
    `M${sx(xMin)},${sy(0)}` + pts.map(([t, v]) => `H${sx(t)}V${sy(v)}`).join("") + `H${sx(xMax)}`;

  function fmtTime(t: number) {
    const d = new Date(t);
    const clock = [d.getHours(), d.getMinutes(), ...(xMax - xMin < 600_000 ? [d.getSeconds()] : [])];
    return `${d.getMonth() + 1}/${d.getDate()} ${clock.map(pad2).join(":")}`;
  }

  let yTicks = $derived.by(() => {
    if (yMax === 0) return [0];
    const raw = yMax / 5;
    const mag = 10 ** Math.floor(Math.log10(raw));
    const step = Math.ceil(raw / mag) * mag;
    return Array.from({ length: Math.floor(yMax / step) + 1 }, (_, i) => i * step);
  });

  let xTicks = $derived.by(() => {
    if (xMax === xMin) return [xMin];
    const count = Math.max(2, Math.min(8, Math.floor(plotW / (7.5 * rem))));
    return Array.from({ length: count + 1 }, (_, i) => xMin + ((xMax - xMin) * i) / count);
  });

  let hovered = $state<string | null>(null);
  let mouseX = $state<number | null>(null);
  const dim = (id: string) => hovered !== null && hovered !== id;

  function onMove(e: MouseEvent & { currentTarget: SVGSVGElement }) {
    const rect = e.currentTarget.getBoundingClientRect();
    const x = ((e.clientX - rect.left) / rect.width) * W;
    mouseX = x < pad.left || x > W - pad.right ? null : x;
  }

  let tooltip = $derived.by(() => {
    if (mouseX === null) return [];
    const t = xMin + ((mouseX - pad.left) / plotW) * (xMax - xMin);
    return series
      .map((s) => ({
        name: s.name,
        color: s.color,
        score: s.points.findLast((p) => p[0] <= t)?.[1] ?? 0,
      }))
      .filter((d) => d.score > 0)
      .sort((a, b) => b.score - a.score);
  });
</script>

<svelte:window onresize={() => (rem = readRem())} />

<div class="mx-6 mt-6 card p-5">
  <div class="flex items-center justify-between mb-4">
    <div class="card-title">Score over Time</div>
    <div class="flex flex-wrap gap-x-4 gap-y-1">
      {#each series as s (s.id)}
        <button
          class="flex items-center gap-1.5 text-label bg-transparent border-none cursor-pointer p-0
          transition-opacity duration-150"
          class:opacity-25={dim(s.id)}
          style:color={s.color}
          onmouseenter={() => (hovered = s.id)}
          onmouseleave={() => (hovered = null)}
        >
          <span class="w-3 h-[0.125rem] inline-block rounded-full bg-current"></span>
          {s.name}
        </button>
      {/each}
    </div>
  </div>

  <div class="w-full relative" bind:clientWidth={width}>
    <svg
      role="img"
      aria-label="Team scores over time"
      width={W}
      height={H}
      viewBox="0 0 {W} {H}"
      class="w-full block"
      onmousemove={onMove}
      onmouseleave={() => (mouseX = null)}
    >
      {#each yTicks as tick}
        <line x1={pad.left} y1={sy(tick)} x2={W - pad.right} y2={sy(tick)} class="stroke-text/5" />
        <text x={pad.left - 0.625 * rem} y={sy(tick) + 0.25 * rem} text-anchor="end" class="fill-text/30 text-2xs">
          {tick}
        </text>
      {/each}

      {#each xTicks as tick, i}
        <line x1={sx(tick)} y1={pad.top} x2={sx(tick)} y2={bottom} class="stroke-text/3" />
        <text
          x={sx(tick)}
          y={H - 0.5 * rem}
          text-anchor={i === 0 ? "start" : i === xTicks.length - 1 ? "end" : "middle"}
          class="fill-text/30 text-3xs"
        >
          {fmtTime(tick)}
        </text>
      {/each}

      {#each series as s (s.id)}
        {@const line = stepPath(s.points)}
        <g style:color={s.color} opacity={dim(s.id) ? 0.15 : 1} class="transition-opacity duration-150">
          <linearGradient id="grad-{s.id}" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stop-color="currentColor" stop-opacity="0.15" />
            <stop offset="100%" stop-color="currentColor" stop-opacity="0" />
          </linearGradient>
          <path
            d="{line}V{bottom}Z"
            fill="url(#grad-{s.id})"
            opacity={dim(s.id) ? 0 : 1}
            class="transition-opacity duration-150"
          />
          <path d={line} fill="none" stroke="currentColor" stroke-width={0.125 * rem} />
          {#each s.points as [t, v]}
            <circle cx={sx(t)} cy={sy(v)} r={0.1875 * rem} fill="currentColor" />
          {/each}
        </g>
      {/each}

      {#if mouseX !== null}
        <line x1={mouseX} y1={pad.top} x2={mouseX} y2={bottom} class="stroke-text/20" stroke-dasharray="3,3" />
      {/if}
    </svg>

    {#if mouseX !== null && tooltip.length > 0}
      {@const left = (mouseX / W) * 100}
      {@const flip = left > 70}
      <div
        class="absolute top-2 pointer-events-none bg-background border border-text/10
        rounded-md px-3 py-2 text-label min-w-28 z-10"
        style:left="{flip ? left - 1 : left + 1}%"
        style:transform="translateX({flip ? '-100%' : '0'})"
      >
        {#each tooltip as d}
          <div class="flex items-center justify-between gap-4 py-0.5">
            <div class="flex items-center gap-1.5" style:color={d.color}>
              <span class="w-2 h-2 rounded-full inline-block bg-current"></span>
              <span>{d.name}</span>
            </div>
            <span class="text-text font-bold">{d.score}</span>
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>
