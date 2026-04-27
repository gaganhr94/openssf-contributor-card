// Collapse pill rows that overflow a single line into a "+N" tail.
// Clicking +N expands the row; remeasures on window resize.
(() => {
  const lists = document.querySelectorAll('[data-collapsible="true"]');
  if (!lists.length) return;

  // Reserve room for a "+999"-shaped chip so it never wraps onto a second
  // line itself when we add it.
  const RESERVE_PX = 80;

  function reset(list) {
    Array.from(list.children).forEach(el => {
      if (el.classList.contains('pill-more')) {
        el.remove();
      } else {
        el.hidden = false;
      }
    });
  }

  function collapse(list) {
    if (list.dataset.expanded === '1') return;
    reset(list);

    // No measurement to do yet — likely the row hasn't been laid out.
    if (list.clientWidth === 0) return;

    // scrollWidth is the unclipped row width; clientWidth is the visible
    // width. If the former exceeds the latter, we have overflow.
    if (list.scrollWidth <= list.clientWidth + 1) return;

    const items = Array.from(list.children).filter(el => !el.classList.contains('pill-more'));
    if (items.length <= 1) return;

    const containerWidth = list.clientWidth;

    // Walk forward, recording how many items we can keep visible while
    // leaving room for the +N chip at the end.
    let visibleCount = 0;
    for (let i = 0; i < items.length; i++) {
      const it = items[i];
      const right = it.offsetLeft + it.offsetWidth;
      // Reserve space for +N chip after the last visible item.
      const isLast = i === items.length - 1;
      const needed = isLast ? right : right + RESERVE_PX;
      if (needed > containerWidth) break;
      visibleCount = i + 1;
    }
    // Always show at least one pill so the row isn't just a +N chip.
    if (visibleCount === 0) visibleCount = 1;

    if (visibleCount >= items.length) return; // everything fits

    const hidden = items.slice(visibleCount);
    hidden.forEach(el => { el.hidden = true; });

    const more = document.createElement('li');
    more.className = 'pill pill-more';
    more.tabIndex = 0;
    more.setAttribute('role', 'button');
    more.setAttribute('aria-label', 'Show ' + hidden.length + ' more');
    more.textContent = '+' + hidden.length;
    const expand = () => {
      list.dataset.expanded = '1';
      Array.from(list.querySelectorAll('[hidden]')).forEach(el => { el.hidden = false; });
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
      if (list.dataset.expanded === '1') return;
      collapse(list);
    });
  }

  // Run early and again once everything (including fonts) is settled —
  // pill widths can shift between DOM-ready and load.
  const safeRun = () => requestAnimationFrame(run);
  safeRun();
  if (document.readyState !== 'complete') {
    window.addEventListener('load', safeRun);
  }
  if (document.fonts && document.fonts.ready) {
    document.fonts.ready.then(safeRun);
  }

  let resizeTimer = null;
  window.addEventListener('resize', () => {
    clearTimeout(resizeTimer);
    resizeTimer = setTimeout(() => {
      lists.forEach(l => delete l.dataset.expanded);
      safeRun();
    }, 120);
  });
})();
