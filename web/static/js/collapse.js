// Collapse pill rows that overflow a single line into a "+N" tail.
// Clicking +N expands the row; remeasures on window resize.
(() => {
  const lists = document.querySelectorAll('[data-collapsible="true"]');
  if (!lists.length) return;

  // Reserve room for a "+999"-shaped chip so it never wraps onto a second
  // line itself when we add it.
  const RESERVE_PX = 70;

  function collapse(list) {
    if (list.dataset.expanded === '1') return;
    // Reset prior state: unhide everything, drop any prior +N chip.
    Array.from(list.children).forEach(el => {
      if (el.classList.contains('pill-more')) {
        el.remove();
      } else {
        el.hidden = false;
      }
    });

    const items = Array.from(list.children);
    if (items.length <= 1) return;

    const containerWidth = list.clientWidth;
    let lastFit = -1;
    for (let i = 0; i < items.length; i++) {
      const it = items[i];
      // offsetLeft + offsetWidth gives the right edge inside the parent.
      const right = it.offsetLeft + it.offsetWidth;
      if (right > containerWidth) break;
      lastFit = i;
    }

    if (lastFit === items.length - 1) return; // everything fits

    // Now ensure there's room for the +N chip without overflowing. Walk
    // back items as long as the remaining tail + reserve doesn't fit.
    while (lastFit >= 0) {
      const it = items[lastFit];
      const right = it.offsetLeft + it.offsetWidth;
      if (right + RESERVE_PX <= containerWidth) break;
      lastFit--;
    }

    const hidden = items.slice(lastFit + 1);
    if (!hidden.length) return;
    hidden.forEach(el => { el.hidden = true; });

    const more = document.createElement('li');
    more.className = 'pill pill-more';
    more.tabIndex = 0;
    more.setAttribute('role', 'button');
    more.setAttribute('aria-label', 'Show ' + hidden.length + ' more');
    more.textContent = '+' + hidden.length;
    const expand = () => {
      Array.from(list.querySelectorAll('[hidden]')).forEach(el => { el.hidden = false; });
      list.dataset.expanded = '1';
      more.remove();
    };
    more.addEventListener('click', expand);
    more.addEventListener('keydown', (ev) => {
      if (ev.key === 'Enter' || ev.key === ' ') {
        ev.preventDefault();
        expand();
      }
    });
    list.appendChild(more);
  }

  function run() {
    lists.forEach(list => {
      // Only collapse if the user hasn't manually expanded.
      if (list.dataset.expanded === '1') return;
      collapse(list);
    });
  }

  // Initial pass + on resize (debounced).
  let resizeTimer = null;
  window.addEventListener('resize', () => {
    clearTimeout(resizeTimer);
    resizeTimer = setTimeout(() => {
      // Reset expanded so resize re-collapses.
      lists.forEach(l => delete l.dataset.expanded);
      run();
    }, 120);
  });

  // Run after fonts have settled so width measurements are accurate.
  if (document.readyState === 'complete') {
    run();
  } else {
    window.addEventListener('load', run);
  }
})();
