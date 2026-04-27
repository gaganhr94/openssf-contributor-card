// Substring search over the contributor index embedded in #contrib-data.
// Hides tiles whose login/name/projects don't match. Pure vanilla JS so the
// site needs no JS framework.
(() => {
  const data = JSON.parse(document.getElementById('contrib-data').textContent || '[]');
  const grid = document.getElementById('contributors');
  const empty = document.getElementById('empty');
  const input = document.getElementById('search');
  if (!grid || !input) return;

  // Build an index aligned with the data array. We only render the top-N tiles
  // server-side, so when a search matches a contributor outside the rendered
  // top-N we render a tile on the fly.
  const tiles = new Map(); // login -> element
  for (const tile of grid.querySelectorAll('.contributor-tile')) {
    tiles.set(tile.dataset.login.toLowerCase(), tile);
  }

  const filter = (q) => {
    q = q.trim().toLowerCase();
    if (!q) {
      // Reset: show everything that was rendered server-side, hide overflow.
      grid.querySelectorAll('.contributor-tile').forEach(t => {
        t.hidden = !!t.dataset.dynamic;
        if (t.dataset.dynamic) t.remove();
      });
      empty.hidden = true;
      return;
    }
    let shown = 0;
    // First pass: hide tiles already rendered that don't match.
    grid.querySelectorAll('.contributor-tile:not([data-dynamic])').forEach(t => {
      const hay = (t.dataset.login + ' ' + t.dataset.name + ' ' + t.dataset.projects).toLowerCase();
      const match = hay.includes(q);
      t.hidden = !match;
      if (match) shown++;
    });
    // Second pass: search the full data array for matches not yet visible.
    grid.querySelectorAll('.contributor-tile[data-dynamic]').forEach(t => t.remove());
    if (shown < 60) {
      for (const c of data) {
        if (tiles.has(c.l.toLowerCase())) continue; // already in grid
        const hay = (c.l + ' ' + (c.n || '') + ' ' + (c.p || []).join(' ')).toLowerCase();
        if (!hay.includes(q)) continue;
        const li = document.createElement('li');
        li.className = 'contributor-tile';
        li.dataset.dynamic = '1';
        li.dataset.login = c.l;
        li.dataset.name = c.n || '';
        li.dataset.projects = (c.p || []).join(' ');
        li.innerHTML =
          '<a href="/c/' + encodeURIComponent(c.l) + '.html">' +
          '<div class="tile-name">' + escapeHTML(c.n || c.l) + '</div>' +
          '<div class="tile-login">@' + escapeHTML(c.l) + '</div>' +
          '<div class="tile-stats">' + c.c + ' commits &middot; ' + (c.p || []).length + ' project' + ((c.p || []).length === 1 ? '' : 's') + '</div>' +
          '</a>';
        grid.appendChild(li);
        shown++;
        if (shown >= 60) break;
      }
    }
    empty.hidden = shown > 0;
  };

  const escapeHTML = (s) =>
    s.replace(/[&<>"']/g, ch => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]));

  let t = null;
  input.addEventListener('input', () => {
    clearTimeout(t);
    t = setTimeout(() => filter(input.value), 80);
  });
})();
