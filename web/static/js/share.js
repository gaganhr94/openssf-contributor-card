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

  // Generic helper: writes text to clipboard with a textarea fallback for
  // non-secure contexts.
  async function copyText(text) {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text);
      return;
    }
    const ta = document.createElement('textarea');
    ta.value = text;
    ta.style.position = 'fixed';
    ta.style.opacity = '0';
    document.body.appendChild(ta);
    ta.select();
    document.execCommand('copy');
    ta.remove();
  }

  // Copy-link button: writes URL, briefly swaps the icon to a check.
  if (copyBtn) {
    copyBtn.addEventListener('click', async () => {
      const url = copyBtn.dataset.url;
      if (!url) return;
      try {
        await copyText(url);
        copyBtn.classList.add('copied');
        setTimeout(() => copyBtn.classList.remove('copied'), 1400);
      } catch (e) {
        window.prompt('Copy link:', url);
      }
    });
  }

  // Embed dialog: opens a modal with markdown / HTML snippets that link
  // back to the contributor page via the OG image.
  const embedTrigger = document.getElementById('embed-trigger');
  const embedDialog = document.getElementById('embed-dialog');
  if (embedTrigger && embedDialog) {
    const backdrop = embedDialog.querySelector('.embed-backdrop');
    const closeDialog = embedDialog.querySelector('.embed-dialog-close');

    const openEmbed = () => {
      // Close the share popup first so we don't have two overlays open.
      close();
      embedDialog.hidden = false;
      // Move focus into the dialog for keyboard users.
      const focusable = embedDialog.querySelector('button, [href], input');
      if (focusable) focusable.focus();
    };
    const closeEmbed = () => {
      embedDialog.hidden = true;
      embedDialog.querySelectorAll('.embed-copy.copied').forEach(b => b.classList.remove('copied'));
    };

    embedTrigger.addEventListener('click', (ev) => {
      ev.stopPropagation();
      openEmbed();
    });
    if (closeDialog) closeDialog.addEventListener('click', closeEmbed);
    if (backdrop) backdrop.addEventListener('click', closeEmbed);
    document.addEventListener('keydown', (ev) => {
      if (ev.key === 'Escape' && !embedDialog.hidden) closeEmbed();
    });

    // Per-snippet copy buttons.
    embedDialog.querySelectorAll('.embed-copy').forEach((btn) => {
      btn.addEventListener('click', async () => {
        const targetId = btn.dataset.target;
        const target = targetId ? document.getElementById(targetId) : null;
        if (!target) return;
        try {
          await copyText(target.textContent);
          btn.classList.add('copied');
          btn.textContent = 'Copied';
          setTimeout(() => {
            btn.classList.remove('copied');
            btn.textContent = 'Copy';
          }, 1400);
        } catch (e) {
          window.prompt('Copy snippet:', target.textContent);
        }
      });
    });
  }
})();
