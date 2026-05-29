/**
 * Pins an element to the bottom of the layout viewport (not the visual viewport).
 * On iOS, when the keyboard opens, the visual viewport shrinks but we keep the
 * dock at the physical bottom so it stays behind the keyboard.
 */
export function pinLayoutBottom(node: HTMLElement) {
  const update = () => {
    const layoutHeight = document.documentElement.clientHeight;
    const height = node.offsetHeight;
    node.style.position = 'fixed';
    node.style.left = '0';
    node.style.right = '0';
    node.style.bottom = 'auto';
    node.style.top = `${layoutHeight - height}px`;
    node.style.transform = 'none';
    node.style.willChange = 'top';
  };

  update();

  const vv = window.visualViewport;
  vv?.addEventListener('resize', update);
  vv?.addEventListener('scroll', update);
  window.addEventListener('resize', update);
  window.addEventListener('orientationchange', update);

  const ro = new ResizeObserver(update);
  ro.observe(node);

  return {
    destroy() {
      vv?.removeEventListener('resize', update);
      vv?.removeEventListener('scroll', update);
      window.removeEventListener('resize', update);
      window.removeEventListener('orientationchange', update);
      ro.disconnect();
    },
  };
}
