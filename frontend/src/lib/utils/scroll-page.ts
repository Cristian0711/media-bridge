/** Scroll the main page back to the top (document/body scroll). */
export function scrollPageToTop(): void {
  if (typeof window === 'undefined') return;
  window.scrollTo(0, 0);
  document.documentElement.scrollTop = 0;
  document.body.scrollTop = 0;
}
