// Guide Ibalya : surlignage de la section
// courante dans le sommaire. Le thème est géré par /theme.js. Script externe car la CSP du site interdit
// l'inline (script-src 'self').
(function () {
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
