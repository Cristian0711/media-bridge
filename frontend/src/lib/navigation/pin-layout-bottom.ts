/**
 * Pins an element to the bottom of the viewport.
 * On iOS, when the keyboard opens, uses the visual viewport so inputs stay visible.
 */
export function pinLayoutBottom(node: HTMLElement, alignToVisualViewport = false) {
  let useVisual = alignToVisualViewport;
  let syncTimers: ReturnType<typeof setTimeout>[] = [];

  const clearSyncTimers = () => {
    for (const id of syncTimers) clearTimeout(id);
    syncTimers = [];
  };

  const keyboardLikelyOpen = (): boolean => {
    const vv = window.visualViewport;
    if (!vv) return false;
    const layoutHeight = window.innerHeight;
    return vv.height < layoutHeight - 80 || vv.offsetTop > 0;
  };

  const update = () => {
    const vv = window.visualViewport;
    const layoutHeight = document.documentElement.clientHeight;
    const height = node.offsetHeight;

    node.style.position = 'fixed';
    node.style.left = '0';
    node.style.right = '0';
    node.style.bottom = 'auto';
    node.style.transform = 'none';
    node.style.willChange = 'top';

    const pinToVisual = useVisual || keyboardLikelyOpen();
    if (pinToVisual && vv) {
      node.style.top = `${vv.offsetTop + vv.height - height}px`;
    } else {
      node.style.top = `${layoutHeight - height}px`;
    }
  };

  const scheduleKeyboardSync = () => {
    clearSyncTimers();
    update();
    requestAnimationFrame(() => {
      update();
      requestAnimationFrame(update);
    });
    for (const ms of [50, 120, 250, 400]) {
      syncTimers.push(setTimeout(update, ms));
    }
  };

  update();

  const onViewportChange = () => {
    if (useVisual || keyboardLikelyOpen()) {
      update();
    } else {
      update();
    }
  };

  const vv = window.visualViewport;
  vv?.addEventListener('resize', onViewportChange);
  vv?.addEventListener('scroll', onViewportChange);
  window.addEventListener('resize', onViewportChange);
  window.addEventListener('orientationchange', onViewportChange);

  const ro = new ResizeObserver(() => {
    if (useVisual) {
      scheduleKeyboardSync();
    } else {
      update();
    }
  });
  ro.observe(node);

  return {
    update(alignVisual?: boolean) {
      const next = alignVisual === true;
      const becameVisual = next && !useVisual;
      useVisual = next;
      if (becameVisual) {
        scheduleKeyboardSync();
      } else {
        clearSyncTimers();
        update();
      }
    },
    destroy() {
      clearSyncTimers();
      vv?.removeEventListener('resize', onViewportChange);
      vv?.removeEventListener('scroll', onViewportChange);
      window.removeEventListener('resize', onViewportChange);
      window.removeEventListener('orientationchange', onViewportChange);
      ro.disconnect();
    },
  };
}
