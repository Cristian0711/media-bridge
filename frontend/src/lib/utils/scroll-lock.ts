let lockCount = 0;
let savedScrollY = 0;

/** Prevent background page scroll while a modal is open (supports stacked modals). */
export function lockBodyScroll(): () => void {
  if (typeof document === 'undefined') return () => {};

  if (lockCount === 0) {
    savedScrollY = window.scrollY;
    const { documentElement: html, body } = document;
    html.style.overflow = 'hidden';
    body.style.overflow = 'hidden';
    body.style.position = 'fixed';
    body.style.top = `-${savedScrollY}px`;
    body.style.left = '0';
    body.style.right = '0';
    body.style.width = '100%';
  }

  lockCount += 1;

  return () => {
    lockCount -= 1;
    if (lockCount > 0 || typeof document === 'undefined') return;

    const { documentElement: html, body } = document;
    html.style.overflow = '';
    body.style.overflow = '';
    body.style.position = '';
    body.style.top = '';
    body.style.left = '';
    body.style.right = '';
    body.style.width = '';
    window.scrollTo(0, savedScrollY);
  };
}
