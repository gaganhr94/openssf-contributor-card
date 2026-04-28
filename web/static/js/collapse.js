// Collapse pill rows that overflow a single line into a "+N more" tail.
// Clicking +N expands the row; remeasures on resize.
//
// Strategy: the list uses flex-wrap: wrap so the browser does the layout
// itself, and max-height clips visually to a single row. We then read the
// rendered top of each pill — anything below the first row is overflow.
// Hiding overflow pills on a wrap layout is more robust than measuring
// against an overflowing nowrap row, which fooled earlier attempts.
(() => {
  const lists = document.querySelectorAll('[data-collapsible="true"]');
  if (!lists.length) return;

  function reset(list) {
    Array.from(list.children).forEach(el => {
      if (el.classList.contains('pill-more')) {
        el.remove();
      } else {
        el.hidden = false;
      }
    });
  }

  function makeMoreChip(hiddenCount) {
    const more = document.createElement('li');
    more.className = 'pill pill-more';
    more.tabIndex = 0;
    more.setAttribute('role', 'button');
    more.textContent = '+' + hiddenCount + ' more';
    more.setAttribute('aria-label', 'Show ' + hiddenCount + ' more');
    return more;
  }

  function collapse(list) {
    if (list.dataset.expanded === '1') return;
    reset(list);

    const items = Array.from(list.children).filter(el => !el.classList.contains('pill-more'));
    if (items.length <= 1) return;

    // Read the first pill's top to define "line 1". Anything with a top
    // measurably below that is on line 2+ and must be hidden. We use half
    // a pill height as the line-break threshold — sub-pixel rendering
    // shifts (especially the +N chip's tabular-nums + bold weight) can
    // push neighbours by a pixel or two even when they sit on the same
    // visual row, so a strict 0.5px cutoff misclassified them as line 2.
    const firstRect = items[0].getBoundingClientRect();
    if (firstRect.height === 0 && firstRect.width === 0) return;
    const firstTop = firstRect.top;
    const lineThreshold = Math.max(firstRect.height / 2, 8);

    let visibleCount = items.length;
    for (let i = 1; i < items.length; i++) {
      if (items[i].getBoundingClientRect().top - firstTop > lineThreshold) {
        visibleCount = i;
        break;
      }
    }
    if (visibleCount >= items.length) return; // every pill fits on line 1

    // Hide the overflow pills.
    for (let i = visibleCount; i < items.length; i++) {
      items[i].hidden = true;
    }

    let hiddenCount = items.length - visibleCount;
    const more = makeMoreChip(hiddenCount);
    list.appendChild(more);

    // The +N chip itself takes width — if appending it pushed it onto line
    // 2, hide one more pill at a time until the chip lands on line 1. On
    // very narrow viewports with long pill text, even a single pill can be
    // wider than the row, so we allow visibleCount to drop to 0 and show
    // only the chip rather than letting it wrap.
    let safety = items.length + 1;
    while (safety-- > 0) {
      if (more.getBoundingClientRect().top - firstTop <= lineThreshold) break;
      if (visibleCount <= 0) break;
      visibleCount--;
      items[visibleCount].hidden = true;
      hiddenCount++;
      more.textContent = '+' + hiddenCount + ' more';
      more.setAttribute('aria-label', 'Show ' + hiddenCount + ' more');
    }

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
  }

  function run() {
    lists.forEach(list => {
      if (list.dataset.expanded === '1') return;
      collapse(list);
    });
  }

  // Pill widths can shift between layout, font load, and image load — run
  // at multiple points so we don't miss the final settled layout.
  const safeRun = () => requestAnimationFrame(run);
  safeRun();
  if (document.readyState !== 'complete') {
    window.addEventListener('load', safeRun);
  }
  if (document.fonts && document.fonts.ready) {
    document.fonts.ready.then(safeRun);
  }
  setTimeout(safeRun, 250);
  setTimeout(safeRun, 800);

  // ResizeObserver covers container width changes (window resize, parent
  // layout shifts). Don't fight a user who already expanded the row.
  if (typeof ResizeObserver === 'function') {
    const ro = new ResizeObserver(() => {
      lists.forEach(list => {
        if (list.dataset.expanded === '1') return;
        collapse(list);
      });
    });
    lists.forEach(list => ro.observe(list));
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
