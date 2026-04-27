// Collapse pill rows that overflow a single line into a "+N" tail.
// Clicking +N expands the row; remeasures on resize.
(() => {
  const lists = document.querySelectorAll('[data-collapsible="true"]');
  if (!lists.length) return;

  // Reserve room so the +N chip itself never overflows after we add it.
  const RESERVE_PX = 64;

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

    // Use getBoundingClientRect for accurate sub-pixel measurement —
    // offsetLeft/clientWidth round to ints which has tripped this up before.
    const containerRect = list.getBoundingClientRect();
    if (containerRect.width === 0) return;

    const items = Array.from(list.children).filter(el => !el.classList.contains('pill-more'));
    if (items.length <= 1) return;

    // First, see whether the row would overflow at all by checking the
    // far edge of the last item.
    const last = items[items.length - 1];
    const lastRect = last.getBoundingClientRect();
    const lastRightOffset = lastRect.right - containerRect.left;
    if (lastRightOffset <= containerRect.width + 0.5) {
      return; // everything fits
    }

    // Walk forward and find how many items we can keep visible while
    // leaving room for the +N chip.
    let visibleCount = 0;
    for (let i = 0; i < items.length; i++) {
      const r = items[i].getBoundingClientRect();
      const rightOffset = r.right - containerRect.left;
      const isLast = i === items.length - 1;
      const allowed = isLast ? containerRect.width : containerRect.width - RESERVE_PX;
      if (rightOffset > allowed) break;
      visibleCount = i + 1;
    }
    if (visibleCount === 0) visibleCount = 1; // always show at least one

    if (visibleCount >= items.length) return;

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

  // Trigger run() across multiple timing points: layout, fonts loading,
  // window load. Pill widths can shift between these moments and any
  // single trigger may be early.
  const safeRun = () => requestAnimationFrame(run);
  safeRun();
  if (document.readyState !== 'complete') {
    window.addEventListener('load', safeRun);
  }
  if (document.fonts && document.fonts.ready) {
    document.fonts.ready.then(safeRun);
  }
  // Catch any layout reflow that happens after first paint (late image
  // loads, late CSS, etc.).
  setTimeout(safeRun, 250);
  setTimeout(safeRun, 800);

  // ResizeObserver re-runs whenever the list or its contents change size.
  if (typeof ResizeObserver === 'function') {
    const ro = new ResizeObserver(() => {
      // Don't fight a user who has manually expanded — only re-collapse
      // if we haven't expanded.
      lists.forEach(list => {
        if (list.dataset.expanded === '1') return;
        collapse(list);
      });
    });
    lists.forEach(list => ro.observe(list));
  }

  // Window resize: full reset so the +N count adapts to new width.
  let resizeTimer = null;
  window.addEventListener('resize', () => {
    clearTimeout(resizeTimer);
    resizeTimer = setTimeout(() => {
      lists.forEach(l => delete l.dataset.expanded);
      safeRun();
    }, 120);
  });
})();
