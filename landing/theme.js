// Bascule clair / sombre de la landing. Défaut : clair. Le choix est mémorisé.
// Chargé dans le <head> pour poser le thème avant le premier rendu (pas de
// flash chez qui a choisi le sombre). Script externe car la CSP interdit
// l'inline (script-src 'self').
(function () {
  var root = document.documentElement;

  try {
    var t = localStorage.getItem('ibalya_theme');
    if (t === 'dark' || t === 'light') root.setAttribute('data-theme', t);
  } catch (e) { /* stockage indisponible : on reste sur le clair par défaut */ }

  function bind() {
    var btn = document.getElementById('lp-theme');
    if (!btn) return;
    btn.addEventListener('click', function () {
      var cur = root.getAttribute('data-theme') || 'light';
      var next = cur === 'dark' ? 'light' : 'dark';
      root.setAttribute('data-theme', next);
      try { localStorage.setItem('ibalya_theme', next); } catch (e) { /* sans effet */ }
    });
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', bind);
  } else {
    bind();
  }
})();
