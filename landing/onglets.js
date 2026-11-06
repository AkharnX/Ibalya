/* Onglets des simulateurs (#simulateurs). Amélioration progressive : sans ce script,
   les six simulateurs restent empilés et la barre d'onglets est masquée par le CSS.
   Script externe, compatible CSP script-src 'self'. */
(function () {
  'use strict';
  var liste = document.querySelector('.sim-onglets');
  var panneau = document.querySelector('.sim-panneau');
  if (!liste || !panneau) return;
  var boutons = Array.prototype.slice.call(liste.querySelectorAll('[data-onglet]'));
  var sims = Array.prototype.slice.call(panneau.querySelectorAll('.sim[data-sim]'));
  if (!boutons.length || !sims.length) return;

  function simDe(id) {
    for (var i = 0; i < sims.length; i++) if (sims[i].getAttribute('data-sim') === id) return sims[i];
    return null;
  }

  boutons.forEach(function (b) {
    var s = simDe(b.getAttribute('data-onglet'));
    if (!s) return;
    b.setAttribute('role', 'tab');
    b.id = b.id || 'onglet-' + b.getAttribute('data-onglet');
    b.setAttribute('aria-controls', s.id);
    s.setAttribute('role', 'tabpanel');
    s.setAttribute('aria-labelledby', b.id);
  });

  function activer(id, focus) {
    boutons.forEach(function (b) {
      var on = b.getAttribute('data-onglet') === id;
      b.setAttribute('aria-selected', on ? 'true' : 'false');
      b.tabIndex = on ? 0 : -1;
      if (on && focus) b.focus();
    });
    sims.forEach(function (s) { s.hidden = s.getAttribute('data-sim') !== id; });
  }

  liste.addEventListener('click', function (e) {
    var b = e.target.closest ? e.target.closest('[data-onglet]') : null;
    if (b) activer(b.getAttribute('data-onglet'));
  });
  liste.addEventListener('keydown', function (e) {
    var i = boutons.indexOf(document.activeElement);
    if (i < 0) return;
    var j = -1;
    if (e.key === 'ArrowRight') j = (i + 1) % boutons.length;
    else if (e.key === 'ArrowLeft') j = (i - 1 + boutons.length) % boutons.length;
    else if (e.key === 'Home') j = 0;
    else if (e.key === 'End') j = boutons.length - 1;
    if (j < 0) return;
    e.preventDefault();
    activer(boutons[j].getAttribute('data-onglet'), true);
  });

  liste.classList.add('sim-onglets-actifs');
  activer(boutons[0].getAttribute('data-onglet'));
})();
