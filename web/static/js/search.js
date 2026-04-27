// Username-search dropdown for the index page.
//
// Behaviour: type → list of matching contributors appears as a dropdown.
// Enter, click, or up/down + Enter navigates to /c/<login>.html.
// Pure vanilla, no framework.
(() => {
  const data = JSON.parse(document.getElementById('contrib-data').textContent || '[]');
  const input = document.getElementById('search');
  const list = document.getElementById('suggestions');
  // Set by the index template; "/<repo>" on project Pages, "" on root deploys.
  const basePath = (typeof window.__BASE_PATH__ === 'string') ? window.__BASE_PATH__ : '';
  if (!input || !list) return;

  const MAX_RESULTS = 8;
  let active = -1;
  let current = []; // currently rendered matches

  const escape = (s) =>
    s.replace(/[&<>"']/g, ch => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]));

  const cardURL = (login) => basePath + '/c/' + encodeURIComponent(login) + '.html';

  // Score: prefix match on login > prefix match on name > substring match.
  // Returns negative score so we can sort ascending; lower is better.
  const score = (entry, q) => {
    const login = entry.l.toLowerCase();
    const name = (entry.n || '').toLowerCase();
    if (login === q) return -1000;
    if (login.startsWith(q)) return -500 + (login.length - q.length);
    if (name.startsWith(q)) return -300 + (name.length - q.length);
    if (login.includes(q)) return -100 + login.indexOf(q);
    if (name.includes(q)) return -50 + name.indexOf(q);
    return 1; // no match
  };

  const filter = (q) => {
    q = q.trim().toLowerCase();
    if (!q) {
      hide();
      return;
    }
    const ranked = [];
    for (const e of data) {
      const s = score(e, q);
      if (s <= 0) ranked.push({ s, e });
    }
    ranked.sort((a, b) => a.s - b.s);
    current = ranked.slice(0, MAX_RESULTS).map(r => r.e);
    render();
  };

  const render = () => {
    if (!current.length) {
      list.innerHTML = '<li class="empty-suggestions">No contributors match. Try a partial GitHub username.</li>';
      list.hidden = false;
      input.setAttribute('aria-expanded', 'true');
      active = -1;
      return;
    }
    list.innerHTML = current.map((c, i) => {
      const avatar = c.a
        ? '<img class="suggestion-avatar" src="' + escape(c.a) + '&s=64" alt="" loading="lazy" width="28" height="28">'
        : '<span class="suggestion-avatar suggestion-avatar-fallback" aria-hidden="true">' + escape((c.l || '?').charAt(0).toUpperCase()) + '</span>';
      return '<li role="option" id="opt-' + i + '" class="suggestion" data-login="' + escape(c.l) + '"' +
        (i === active ? ' aria-selected="true"' : '') + '>' +
        avatar +
        '<span class="suggestion-text">' +
          '<span class="suggestion-name">' + escape(c.n || c.l) + '</span>' +
          '<span class="suggestion-login">@' + escape(c.l) + '</span>' +
        '</span>' +
        '<span class="suggestion-meta">' + c.c + ' contribution' + (c.c === 1 ? '' : 's') + '</span>' +
        '</li>';
    }).join('');
    list.hidden = false;
    input.setAttribute('aria-expanded', 'true');
    if (active >= 0 && active < current.length) {
      input.setAttribute('aria-activedescendant', 'opt-' + active);
    } else {
      input.removeAttribute('aria-activedescendant');
    }
  };

  const hide = () => {
    list.hidden = true;
    list.innerHTML = '';
    input.setAttribute('aria-expanded', 'false');
    input.removeAttribute('aria-activedescendant');
    active = -1;
    current = [];
  };

  const navigate = (login) => {
    if (!login) return;
    window.location.href = cardURL(login);
  };

  // Mouse: click on a suggestion.
  list.addEventListener('mousedown', (ev) => {
    const li = ev.target.closest('.suggestion');
    if (!li) return;
    ev.preventDefault();           // prevent input losing focus before navigation
    navigate(li.dataset.login);
  });

  // Keyboard: arrow keys + enter.
  input.addEventListener('keydown', (ev) => {
    if (list.hidden) return;
    if (ev.key === 'ArrowDown') {
      ev.preventDefault();
      active = Math.min(active + 1, current.length - 1);
      render();
    } else if (ev.key === 'ArrowUp') {
      ev.preventDefault();
      active = Math.max(active - 1, -1);
      render();
    } else if (ev.key === 'Enter') {
      ev.preventDefault();
      if (active >= 0 && active < current.length) {
        navigate(current[active].l);
      } else if (current.length > 0) {
        navigate(current[0].l);     // top match
      } else {
        // Best-effort: try the raw input as a login.
        const raw = input.value.trim();
        if (raw) navigate(raw);
      }
    } else if (ev.key === 'Escape') {
      hide();
    }
  });

  let t = null;
  input.addEventListener('input', () => {
    active = -1;
    clearTimeout(t);
    t = setTimeout(() => filter(input.value), 60);
  });

  input.addEventListener('blur', () => {
    // Delay so a click on a suggestion still registers.
    setTimeout(hide, 120);
  });
  input.addEventListener('focus', () => {
    if (input.value.trim()) filter(input.value);
  });
})();
