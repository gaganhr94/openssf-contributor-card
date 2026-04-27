// Share popup + copy-to-clipboard for the contributor card.
//
// The popup is an inline replacement for the Share trigger: opening it
// hides the trigger and reveals a horizontal pill of icons in its place,
// so card content above is never blocked.
(() => {
  const wrap = document.getElementById('share-wrap');
  const trigger = document.getElementById('share-trigger');
  const popup = document.getElementById('share-popup');
  if (!wrap || !trigger || !popup) return;

  const closeBtn = popup.querySelector('.share-btn-close');
  const copyBtn = popup.querySelector('[data-share="copy"]');

  const open = () => {
    popup.hidden = false;
    wrap.classList.add('open');
    trigger.setAttribute('aria-expanded', 'true');
  };
  const close = () => {
    wrap.classList.remove('open');
    trigger.setAttribute('aria-expanded', 'false');
    popup.hidden = true;
    if (copyBtn) copyBtn.classList.remove('copied');
  };

  trigger.addEventListener('click', (ev) => {
    ev.stopPropagation();
    if (popup.hidden) open(); else close();
  });

  if (closeBtn) closeBtn.addEventListener('click', close);

  document.addEventListener('click', (ev) => {
    if (popup.hidden) return;
    if (popup.contains(ev.target) || trigger.contains(ev.target)) return;
    close();
  });

  document.addEventListener('keydown', (ev) => {
    if (ev.key === 'Escape' && !popup.hidden) close();
  });

  // Social share anchors close the popup as the new tab opens.
  popup.querySelectorAll('a.share-btn').forEach((a) => {
    a.addEventListener('click', () => {
      setTimeout(close, 100);
    });
  });

  // Copy-link button: writes URL, briefly swaps the icon to a check.
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
        copyBtn.classList.add('copied');
        setTimeout(() => copyBtn.classList.remove('copied'), 1400);
      } catch (e) {
        window.prompt('Copy link:', url);
      }
    });
  }
})();
