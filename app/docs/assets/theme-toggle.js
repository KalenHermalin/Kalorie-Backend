/* Shared dark-mode toggle for the Kalorie marketing/docs pages. */
(function () {
  var root = document.documentElement;
  var toggle = document.getElementById('theme-toggle');
  if (!toggle) return;
  var sunIcon = document.getElementById('icon-sun');
  var moonIcon = document.getElementById('icon-moon');

  function systemPrefersDark() {
    return window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches;
  }

  function isDark() {
    var explicit = root.getAttribute('data-theme');
    if (explicit === 'dark') return true;
    if (explicit === 'light') return false;
    return systemPrefersDark();
  }

  function syncIcons() {
    var dark = isDark();
    if (sunIcon) sunIcon.style.display = dark ? 'block' : 'none';
    if (moonIcon) moonIcon.style.display = dark ? 'none' : 'block';
  }

  try {
    var stored = localStorage.getItem('kalorie-theme');
    if (stored === 'dark' || stored === 'light') {
      root.setAttribute('data-theme', stored);
    }
  } catch (e) {}

  syncIcons();

  toggle.addEventListener('click', function () {
    var next = isDark() ? 'light' : 'dark';
    root.setAttribute('data-theme', next);
    try { localStorage.setItem('kalorie-theme', next); } catch (e) {}
    syncIcons();
  });
})();
