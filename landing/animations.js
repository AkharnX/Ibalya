/* Révélation douce au scroll. Amélioration progressive : sans ce script,
   tout le contenu reste visible (les classes sont posées ici, pas dans le HTML). */
(function () {
  'use strict';
  if (window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
  if (!('IntersectionObserver' in window)) return;
  var els = document.querySelectorAll('[data-reveal]');
  if (!els.length) return;
  var io = new IntersectionObserver(function (entries) {
    entries.forEach(function (e) {
      if (e.isIntersecting) {
        e.target.classList.add('is-visible');
        io.unobserve(e.target);
      }
    });
  }, { threshold: 0.12, rootMargin: '0px 0px -6% 0px' });
  els.forEach(function (el) {
    if (el.classList.contains('reveal')) return;
    el.classList.add('reveal');
    io.observe(el);
  });
})();
