export type TabRect = {
  left: number;
  width: number;
  center: number;
};

export function tabRects(track: HTMLElement): TabRect[] {
  const trackBox = track.getBoundingClientRect();
  const items = [...track.querySelectorAll<HTMLElement>('[data-liquid-tab]')];
  return items.map((el) => {
    const box = el.getBoundingClientRect();
    const width = box.width;
    const left = box.left - trackBox.left;
    return { left, width, center: left + width / 2 };
  });
}

export function indexAtX(rects: TabRect[], clientX: number, trackLeft: number): number {
  if (rects.length === 0) return 0;
  const x = clientX - trackLeft;
  for (let i = 0; i < rects.length; i++) {
    const r = rects[i];
    if (x >= r.left && x <= r.left + r.width) return i;
  }
  let nearest = 0;
  let minDist = Infinity;
  for (let i = 0; i < rects.length; i++) {
    const d = Math.abs(x - rects[i].center);
    if (d < minDist) {
      minDist = d;
      nearest = i;
    }
  }
  return nearest;
}

/** Interpolate bubble position/size between tab cells for a liquid slide. */
export function bubbleMetricsAtX(
  rects: TabRect[],
  clientX: number,
  trackLeft: number,
): { left: number; width: number; previewIndex: number } {
  if (rects.length === 0) return { left: 0, width: 0, previewIndex: 0 };
  if (rects.length === 1) {
    return { left: rects[0].left, width: rects[0].width, previewIndex: 0 };
  }

  const x = clientX - trackLeft;
  const previewIndex = indexAtX(rects, clientX, trackLeft);

  if (x <= rects[0].center) {
    return { left: rects[0].left, width: rects[0].width, previewIndex: 0 };
  }
  const last = rects.length - 1;
  if (x >= rects[last].center) {
    return { left: rects[last].left, width: rects[last].width, previewIndex: last };
  }

  for (let i = 0; i < last; i++) {
    const a = rects[i];
    const b = rects[i + 1];
    if (x <= b.center) {
      const span = b.center - a.center || 1;
      const t = Math.max(0, Math.min(1, (x - a.center) / span));
      const eased = t * t * (3 - 2 * t);
      return {
        left: a.left + (b.left - a.left) * eased,
        width: a.width + (b.width - a.width) * eased,
        previewIndex: eased < 0.5 ? i : i + 1,
      };
    }
  }

  return { left: rects[last].left, width: rects[last].width, previewIndex: last };
}
