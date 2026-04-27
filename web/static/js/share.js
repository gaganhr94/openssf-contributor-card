// Copy-to-clipboard for the contributor card share strip.
(() => {
  const btn = document.querySelector('[data-share="copy"]');
  const tip = document.getElementById('copy-tooltip');
  if (!btn) return;

  btn.addEventListener('click', async () => {
    const url = btn.dataset.url;
    if (!url) return;
    try {
      if (navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(url);
      } else {
        // Fallback for non-HTTPS or older browsers.
        const ta = document.createElement('textarea');
        ta.value = url;
        ta.style.position = 'fixed';
        ta.style.opacity = '0';
        document.body.appendChild(ta);
        ta.select();
        document.execCommand('copy');
        ta.remove();
      }
      if (tip) {
        tip.classList.add('visible');
        setTimeout(() => tip.classList.remove('visible'), 1400);
      }
    } catch (e) {
      // Last-ditch: open the URL so the user can copy from the address bar.
      window.prompt('Copy link:', url);
    }
  });
})();
