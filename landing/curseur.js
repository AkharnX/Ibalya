/* Curseur fluide, repris du modèle : un point qui suit la souris, un anneau
   qui le rejoint en retard, les deux grossissent sur les éléments cliquables.
   Amélioration progressive : rien n'est ajouté sans souris fine, ni si
   l'utilisateur préfère moins de mouvement. Le curseur natif reste affiché. */
(function () {
  'use strict';
  if (!window.matchMedia) return;
  if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
  if (!window.matchMedia('(hover: hover) and (pointer: fine)').matches) return;

  var dot = document.createElement('div'); dot.className = 'lp-curseur-point';
  var ring = document.createElement('div'); ring.className = 'lp-curseur-anneau';
  dot.setAttribute('aria-hidden', 'true'); ring.setAttribute('aria-hidden', 'true');
  document.body.appendChild(dot); document.body.appendChild(ring);

  var mx = -100, my = -100, rx = -100, ry = -100, actif = false;
  document.addEventListener('mousemove', function (e) {
    mx = e.clientX; my = e.clientY;
    dot.style.transform = 'translate(' + (mx - 4) + 'px,' + (my - 4) + 'px)' + (actif ? ' scale(1.8)' : '');
    if (!dot.classList.contains('is-visible')) { dot.classList.add('is-visible'); ring.classList.add('is-visible'); }
  });
  document.addEventListener('mouseleave', function () { dot.classList.remove('is-visible'); ring.classList.remove('is-visible'); });
  document.addEventListener('mouseenter', function () { dot.classList.add('is-visible'); ring.classList.add('is-visible'); });

  (function suivre() {
    rx += (mx - rx) * 0.12; ry += (my - ry) * 0.12;
    ring.style.transform = 'translate(' + (rx - 16) + 'px,' + (ry - 16) + 'px)' + (actif ? ' scale(1.5)' : '');
    window.requestAnimationFrame(suivre);
  })();

  /* Délégation : les simulateurs créent leurs boutons après coup. */
  var CIBLES = 'a, button, summary, label, input, select, textarea, [role="button"], [role="tab"]';
  document.addEventListener('mouseover', function (e) {
    var sur = e.target.closest && e.target.closest(CIBLES);
    if (!!sur === actif) return;
    actif = !!sur;
    dot.classList.toggle('is-actif', actif); ring.classList.toggle('is-actif', actif);
    dot.style.transform = 'translate(' + (mx - 4) + 'px,' + (my - 4) + 'px)' + (actif ? ' scale(1.8)' : '');
  });
})();
