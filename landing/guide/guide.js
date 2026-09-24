// Guide Ibalya : bascule de thème (mémorisée) et surlignage de la section
// courante dans le sommaire. Script externe car la CSP du site interdit
// l'inline (script-src 'self').
(function () {
  var root = document.documentElement,
      btn = document.getElementById('themeBtn');

  try {
    var s = localStorage.getItem('guide_theme');
    if (s === 'dark' || s === 'light') root.setAttribute('data-theme', s);
  } catch (e) { /* stockage indisponible : on garde le thème système */ }

  if (btn) {
    btn.addEventListener('click', function () {
      var cur = root.getAttribute('data-theme');
      if (!cur) cur = (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) ? 'dark' : 'light';
      var next = cur === 'dark' ? 'light' : 'dark';
      root.setAttribute('data-theme', next);
      try { localStorage.setItem('guide_theme', next); } catch (e) { /* sans effet */ }
    });
  }

  var links = [].slice.call(document.querySelectorAll('#toc a'));
  if ('IntersectionObserver' in window) {
    var obs = new IntersectionObserver(function (ents) {
      ents.forEach(function (en) {
        if (en.isIntersecting) {
          var id = en.target.id;
          links.forEach(function (a) {
            a.classList.toggle('on', a.getAttribute('href') === '#' + id);
          });
        }
      });
    }, { rootMargin: '-45% 0px -50% 0px' });
    links.forEach(function (a) {
      var el = document.getElementById(a.getAttribute('href').slice(1));
      if (el) obs.observe(el);
    });
  }
})();
