<script lang="ts">
  // Svelte 5 port of LiquidGlassNav from webgl-liquid-glass
  // (https://github.com/clayharmon/webgl-liquid-glass), (c) 2026 Clay Harmon, MIT.
  // The WebGL engine (renderer/shaders/spring/motion) is reused verbatim; this
  // file replaces the React component shell. Positioning is left to the parent
  // (no fixed positioning) so it drops into the app's bottom chrome.
  import { onMount } from 'svelte';
  import { LiquidGlassRenderer } from './renderer';
  import { createSpring, updateSpring } from './spring';
  import { createMotionTracker } from './motion';
  import type { NavItem } from './types';

  interface Props {
    items: NavItem[];
    activeId: string;
    onSelect: (id: string) => void;
    activeColor?: string;
    inactiveColor?: string;
    showLabels?: boolean;
    class?: string;
  }

  let {
    items,
    activeId,
    onSelect,
    activeColor = 'rgba(255, 255, 255, 0.95)',
    inactiveColor = 'rgba(255, 255, 255, 0.5)',
    showLabels = false,
    class: className,
  }: Props = $props();

  const NAV_HEIGHT = 64;
  const NAV_RADIUS = 32;
  const PILL_HEIGHT = 52;
  const DRAG_THRESHOLD = 4;
  const VEL_SMOOTHING = 0.25;
  const ICON_SIZE = 26;

  // --- Color parsing (any CSS color -> normalized rgb) ---
  let parseCtx: CanvasRenderingContext2D | null = null;
  function parseColor(color: string): [number, number, number] {
    if (!parseCtx) parseCtx = document.createElement('canvas').getContext('2d');
    if (!parseCtx) return [1, 1, 1];
    parseCtx.fillStyle = '#000';
    parseCtx.fillStyle = color;
    const c = parseCtx.fillStyle;
    if (c.startsWith('#')) {
      return [
        parseInt(c.slice(1, 3), 16) / 255,
        parseInt(c.slice(3, 5), 16) / 255,
        parseInt(c.slice(5, 7), 16) / 255,
      ];
    }
    const m = c.match(/[\d.]+/g);
    if (m && m.length >= 3) return [Number(m[0]) / 255, Number(m[1]) / 255, Number(m[2]) / 255];
    return [1, 1, 1];
  }

  // --- Refs ---
  let containerEl = $state<HTMLElement | null>(null);
  let canvasEl = $state<HTMLCanvasElement | null>(null);
  let pillEl = $state<HTMLDivElement | null>(null);
  let activeClipEl = $state<HTMLDivElement | null>(null);
  const itemEls: (HTMLButtonElement | null)[] = [];

  // --- Springs (created once) ---
  const springX = createSpring(0);
  const springW = createSpring(0);
  const springSkew = createSpring(0);
  const springBulge = createSpring(0);

  // --- Imperative (non-reactive) state read inside the rAF loop ---
  // (set from activeId in onMount / targetTab — not initialized from the prop
  // directly, which would only capture its first value.)
  let initialized = false;
  let visualTarget = '';
  let navWidth = 0;

  interface DragState {
    active: boolean;
    moved: boolean;
    pointerId: number;
    startPointerX: number;
    pillStartX: number;
    currentX: number;
    velocity: number;
    prevX: number;
  }
  let drag: DragState = emptyDrag();
  function emptyDrag(): DragState {
    return {
      active: false,
      moved: false,
      pointerId: -1,
      startPointerX: 0,
      pillStartX: 0,
      currentX: 0,
      velocity: 0,
      prevX: 0,
    };
  }

  function targetTab(tabId: string) {
    const container = containerEl;
    if (!container) return;
    const idx = items.findIndex((i) => i.id === tabId);
    const btn = itemEls[idx];
    if (!btn) return;

    const cRect = container.getBoundingClientRect();
    const bRect = btn.getBoundingClientRect();
    const x = bRect.left - cRect.left + bRect.width / 2;
    const w = bRect.width;

    springX.target = x;
    springW.target = w;
    visualTarget = tabId;
    navWidth = cRect.width;

    if (!initialized) {
      springX.current = x;
      springW.current = w;
      initialized = true;
    }
  }

  function findTabAt(clientX: number, clientY: number): number {
    for (let i = 0; i < itemEls.length; i++) {
      const btn = itemEls[i];
      if (!btn) continue;
      const r = btn.getBoundingClientRect();
      if (clientX >= r.left && clientX <= r.right && clientY >= r.top && clientY <= r.bottom) return i;
    }
    return -1;
  }

  function findNearestTab(): string | null {
    const container = containerEl;
    if (!container) return null;
    const cRect = container.getBoundingClientRect();
    const x = drag.currentX;

    let bestId: string | null = null;
    let bestDist = Infinity;
    itemEls.forEach((btn, i) => {
      if (!btn) return;
      const bRect = btn.getBoundingClientRect();
      const center = bRect.left - cRect.left + bRect.width / 2;
      const dist = Math.abs(x - center);
      if (dist < bestDist) {
        bestDist = dist;
        bestId = items[i]?.id ?? null;
      }
    });
    return bestId;
  }

  function onPointerDown(e: PointerEvent) {
    const container = containerEl;
    if (!container) return;

    const tabIdx = findTabAt(e.clientX, e.clientY);
    if (tabIdx < 0) return;
    const pressedItem = items[tabIdx];
    if (!pressedItem) return;

    container.setPointerCapture(e.pointerId);
    targetTab(pressedItem.id);

    const cRect = container.getBoundingClientRect();
    const bRect = itemEls[tabIdx]!.getBoundingClientRect();
    const tabCenterX = bRect.left - cRect.left + bRect.width / 2;

    drag = {
      active: true,
      moved: false,
      pointerId: e.pointerId,
      startPointerX: e.clientX,
      pillStartX: tabCenterX,
      currentX: tabCenterX,
      velocity: 0,
      prevX: e.clientX,
    };
    springBulge.target = 1;
  }

  function onPointerMove(e: PointerEvent) {
    const d = drag;
    if (!d.active || e.pointerId !== d.pointerId) return;
    const container = containerEl;
    if (!container) return;
    const cRect = container.getBoundingClientRect();

    const dx = e.clientX - d.startPointerX;
    if (!d.moved && Math.abs(dx) > DRAG_THRESHOLD) d.moved = true;

    if (d.moved) {
      d.currentX = d.pillStartX + dx;
      const halfW = springW.current / 2;
      d.currentX = Math.max(halfW + 8, Math.min(cRect.width - halfW - 8, d.currentX));
    }

    const rawVel = e.clientX - d.prevX;
    d.velocity = d.velocity * (1 - VEL_SMOOTHING) + rawVel * VEL_SMOOTHING;
    d.prevX = e.clientX;
  }

  function onPointerUp(e: PointerEvent) {
    const d = drag;
    if (!d.active || e.pointerId !== d.pointerId) return;
    d.active = false;
    springBulge.target = 0;

    const finalTab = d.moved ? findNearestTab() : visualTarget;
    if (finalTab) onSelect(finalTab);
  }

  // Keyboard activation (pointer selection happens in onPointerUp). Using keydown
  // rather than click avoids a double-fire with the pointer path on touch/mouse.
  function onKeydown(e: KeyboardEvent, id: string) {
    if (e.key !== 'Enter' && e.key !== ' ') return;
    e.preventDefault();
    onSelect(id);
  }

  onMount(() => {
    const canvas = canvasEl;
    const container = containerEl;
    if (!canvas || !container) return;

    let renderer: LiquidGlassRenderer;
    try {
      renderer = new LiquidGlassRenderer(canvas);
    } catch {
      // WebGL unavailable — the DOM pill + backdrop-filter still work without it.
      return;
    }
    const motion = createMotionTracker(container);
    visualTarget = activeId;

    let lastTime = performance.now();
    let frame: number;
    let running = true;

    const ro = new ResizeObserver(() => {
      const rect = container.getBoundingClientRect();
      renderer.resize(rect.width, rect.height);
      navWidth = rect.width;
      targetTab(activeId);
    });
    ro.observe(container);

    const rect = container.getBoundingClientRect();
    renderer.resize(rect.width, rect.height);
    navWidth = rect.width;
    requestAnimationFrame(() => targetTab(activeId));

    function loop() {
      if (!running) return;
      const now = performance.now();
      const dt = Math.min((now - lastTime) / 1000, 0.064);
      lastTime = now;

      const d = drag;

      updateSpring(springBulge, dt, 260, 16);
      springSkew.target = d.active && d.moved ? d.velocity * 0.6 : 0;
      updateSpring(springSkew, dt, 150, 7);

      const bulge = springBulge.current;
      const skew = springSkew.current;

      if (d.active && d.moved) {
        springX.current = d.currentX;
        const targetVel = d.velocity / Math.max(dt, 0.001);
        springX.velocity += (targetVel - springX.velocity) * 0.15;
        springW.current = springW.target;
      } else {
        updateSpring(springX, dt);
        updateSpring(springW, dt);
      }

      const absSkew = Math.abs(skew);
      const morphScaleX = 1 + bulge * 0.12 + Math.min(absSkew * 0.005, 0.2);
      const morphScaleY = 1 + bulge * 0.55 - Math.min(absSkew * 0.003, 0.12);
      const morphSkewDeg = Math.max(-14, Math.min(14, skew * 0.35));

      if (pillEl) {
        const pillX = springX.current - springW.current / 2;
        pillEl.style.transform = `translateX(${pillX}px) scaleX(${morphScaleX}) scaleY(${morphScaleY}) skewX(${morphSkewDeg}deg)`;
        pillEl.style.width = `${springW.current}px`;
      }

      if (activeClipEl) {
        const effW = springW.current * morphScaleX;
        const effH = PILL_HEIGHT * morphScaleY;
        const cw = navWidth;
        const effLeft = springX.current - effW / 2;
        const effTop = (NAV_HEIGHT - effH) / 2;
        const effRight = cw - effLeft - effW;
        const effBottom = NAV_HEIGHT - effTop - effH;
        const effRadius = Math.min(effW, effH) / 2;
        activeClipEl.style.clipPath = `inset(${effTop}px ${effRight}px ${effBottom}px ${effLeft}px round ${effRadius}px)`;
      }

      renderer.render({
        time: now / 1000,
        lightPos: motion.lightPos,
        pillX: springX.current,
        pillWidth: springW.current * morphScaleX,
        pillHeight: PILL_HEIGHT * morphScaleY,
        navRadius: NAV_RADIUS,
        transitionVel: springX.velocity,
        pressAmt: bulge,
        tintColor: parseColor(activeColor),
      });

      frame = requestAnimationFrame(loop);
    }
    frame = requestAnimationFrame(loop);

    return () => {
      running = false;
      cancelAnimationFrame(frame);
      ro.disconnect();
      motion.destroy();
      renderer.destroy();
    };
  });

  // Re-target when the active tab changes from outside (e.g. route change).
  $effect(() => {
    activeId;
    targetTab(activeId);
  });
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<nav
  bind:this={containerEl}
  class={className}
  aria-label="Main navigation"
  style:height="{NAV_HEIGHT}px"
  style:border-radius="{NAV_RADIUS}px"
  onpointerdown={onPointerDown}
  onpointermove={onPointerMove}
  onpointerup={onPointerUp}
  onpointercancel={onPointerUp}
>
  <!-- Spring-animated pill (the active indicator) -->
  <div
    bind:this={pillEl}
    class="lg-pill"
    style:top="{(NAV_HEIGHT - PILL_HEIGHT) / 2}px"
    style:height="{PILL_HEIGHT}px"
    style:border-radius="{PILL_HEIGHT / 2}px"
    aria-hidden="true"
  ></div>

  <!-- WebGL specular / chromatic / shimmer overlay -->
  <canvas bind:this={canvasEl} class="lg-canvas" style:border-radius="{NAV_RADIUS}px" aria-hidden="true"
  ></canvas>

  <!-- Inactive layer (always visible, dim) -->
  {#each items as item, i (item.id)}
    {@const Icon = item.icon}
    <button
      bind:this={itemEls[i]}
      type="button"
      class="lg-item"
      style:color={inactiveColor}
      aria-label={item.label}
      aria-current={activeId === item.id ? 'page' : undefined}
      onkeydown={(e) => onKeydown(e, item.id)}
    >
      {#if Icon}<Icon size={ICON_SIZE} />{/if}
      {#if showLabels}<span class="lg-label">{item.label}</span>{/if}
    </button>
  {/each}

  <!-- Active layer (bright, clipped to the pill shape) -->
  <div bind:this={activeClipEl} class="lg-active" aria-hidden="true">
    {#each items as item (item.id)}
      {@const Icon = item.icon}
      <div class="lg-item lg-item--active" style:color={activeColor}>
        {#if Icon}<Icon size={ICON_SIZE} />{/if}
        {#if showLabels}<span class="lg-label">{item.label}</span>{/if}
      </div>
    {/each}
  </div>
</nav>

<style>
  nav {
    position: relative;
    display: flex;
    align-items: center;
    padding: 0 10px;
    overflow: visible;
    /* Frosted glass: strong blur + saturation so content scrolling behind the
       bar reads as a blurred frost, with a heavier tint so it's less see-through. */
    backdrop-filter: blur(40px) saturate(180%);
    -webkit-backdrop-filter: blur(40px) saturate(180%);
    background: rgb(255 255 255 / 0.3);
    box-shadow:
      0 8px 32px rgb(0 0 0 / 0.25),
      inset 0 0 0 0.5px rgb(255 255 255 / 0.3);
    user-select: none;
    -webkit-user-select: none;
    -webkit-touch-callout: none;
    touch-action: none;
    cursor: pointer;
  }

  .lg-pill {
    position: absolute;
    left: 0;
    background: rgb(255 255 255 / 0.08);
    backdrop-filter: brightness(1.2) saturate(1.4);
    -webkit-backdrop-filter: brightness(1.2) saturate(1.4);
    box-shadow:
      inset 0 1px 1px rgb(255 255 255 / 0.25),
      inset 0 -0.5px 1px rgb(255 255 255 / 0.1),
      0 4px 16px rgb(0 0 0 / 0.1);
    pointer-events: none;
    will-change: transform, width;
    transform-origin: center center;
  }

  .lg-canvas {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    pointer-events: none;
  }

  .lg-item {
    position: relative;
    z-index: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 2px;
    padding: 8px 20px;
    border: none;
    background: none;
    font-size: 10px;
    line-height: 1;
    cursor: pointer;
    -webkit-tap-highlight-color: transparent;
    outline: none;
  }

  .lg-item:focus-visible {
    outline: none;
    box-shadow: 0 0 0 3px rgb(255 255 255 / 0.35);
    border-radius: 9999px;
  }

  .lg-item--active {
    font-weight: 600;
  }

  .lg-active {
    position: absolute;
    inset: 0;
    z-index: 2;
    display: flex;
    align-items: center;
    padding: 0 8px;
    pointer-events: none;
    clip-path: inset(100% 100% 100% 100%);
  }

  .lg-label {
    font-weight: inherit;
  }
</style>
