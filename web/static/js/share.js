// Share popup + copy-to-clipboard for the contributor card.
(() => {
  const trigger = document.getElementById('share-trigger');
  const popup = document.getElementById('share-popup');
  const closeBtn = popup ? popup.querySelector('.share-popup-close') : null;
  const copyBtn = document.querySelector('[data-share="copy"]');
  const tip = document.getElementById('copy-tooltip');
  if (!trigger || !popup) return;

  const open = () => {
    popup.hidden = false;
    trigger.setAttribute('aria-expanded', 'true');
    requestAnimationFrame(() => popup.classList.add('open'));
  };
  const close = () => {
    popup.classList.remove('open');
    trigger.setAttribute('aria-expanded', 'false');
    // Wait out the transition before hiding entirely so screen readers
    // don't pick up content that's still animating away.
    setTimeout(() => { popup.hidden = true; }, 150);
    if (tip) tip.classList.remove('visible');
  };

  trigger.addEventListener('click', (ev) => {
    ev.stopPropagation();
    if (popup.hidden) open(); else close();
  });

  if (closeBtn) closeBtn.addEventListener('click', close);

  // Click outside the popup dismisses.
  document.addEventListener('click', (ev) => {
    if (popup.hidden) return;
    if (popup.contains(ev.target) || trigger.contains(ev.target)) return;
    close();
  });

  // Escape key closes.
  document.addEventListener('keydown', (ev) => {
    if (ev.key === 'Escape' && !popup.hidden) close();
  });

  // Copy-to-clipboard inside the popup.
  if (copyBtn) {
    copyBtn.addEventListener('click', async () => {
      const url = copyBtn.dataset.url;
      if (!url) return;
      try {
        if (navigator.clipboard && window.isSecureContext) {
          await navigator.clipboard.writeText(url);
        } else {
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
        window.prompt('Copy link:', url);
      }
    });
  }

  // Social share links (anchors) close the popup as the new tab opens.
  popup.querySelectorAll('a.share-btn').forEach((a) => {
    a.addEventListener('click', () => {
      // Small delay so the link follow-through isn't disturbed.
      setTimeout(close, 100);
    });
  });
})();
