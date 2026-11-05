/* Ibalya vitrine : socle partagé des simulateurs ROI (section #simulateurs).
   Script externe, sans inline, compatible CSP script-src 'self'.

   Interface exposée sur window.IbSim (consommée par les simulateurs 2 à 7) :
     IbSim.registerSim(id, initFn)   id = valeur de data-sim ('roi','oubli','temps','risque','validation','fiabilite')
                                      initFn(rootEl) appelée UNE fois, quand le .sim entre dans le viewport
     IbSim.fmtEUR(n)  -> "1 234 €"      IbSim.fmtInt(n) -> "1 234"      IbSim.fmtH(n) -> "12,5 h"
     IbSim.animateCount(el, from, to, fmt, durMs?)   écrit fmt(v) dans el.textContent
     IbSim.bindRange(inputEl, onInput) -> getter () => number ; onInput(value, inputEl) appelée à chaque input
     IbSim.svg(tag, attrs, children) -> élément SVG ; children = nœud | chaîne | tableau
     IbSim.prefersReducedMotion      booléen
   Chaque simulateur appelle registerSim après ce script (fichier chargé après simulateurs.js). */
(function () {
  'use strict';
  var NS = 'http://www.w3.org/2000/svg';
  var mq = window.matchMedia ? window.matchMedia('(prefers-reduced-motion: reduce)') : null;
  var reduced = !!(mq && mq.matches);

  function nf(digits) {
    try { return new Intl.NumberFormat('fr-FR', { maximumFractionDigits: digits, minimumFractionDigits: 0 }); }
    catch (e) { return null; }
  }
  var nf0 = nf(0), nf1 = nf(1);
  // Intl fr-FR met une espace fine insécable (U+202F) ; on normalise pour un rendu stable.
  function num(f, n) {
    var s = f ? f.format(n) : String(Math.round(n * 10) / 10).replace('.', ',');
    return s.replace(/[  ]/g, ' ');
  }
  function fmtInt(n) { return num(nf0, Math.round(n)); }
  function fmtEUR(n) { return num(nf0, Math.round(n)) + ' €'; }
  function fmtH(n) { return num(nf1, n) + ' h'; }

  function animateCount(el, from, to, fmt, dur) {
    fmt = fmt || fmtInt;
    if (!isFinite(from)) from = 0;
    if (!isFinite(to)) to = 0;
    // Un nouvel appel sur le même élément annule la boucle précédente (pas d'empilement).
    if (el._ibRaf) { cancelAnimationFrame(el._ibRaf); el._ibRaf = 0; }
    if (reduced || !window.requestAnimationFrame) { el.textContent = fmt(to); return; }
    var d = dur || 700, t0 = null;
    function step(t) {
      if (t0 === null) t0 = t;
      var p = Math.min(1, (t - t0) / d), e = 1 - Math.pow(1 - p, 3);
      el.textContent = fmt(from + (to - from) * e);
      if (p < 1) el._ibRaf = requestAnimationFrame(step);
      else { el._ibRaf = 0; el.textContent = fmt(to); }
    }
    el._ibRaf = requestAnimationFrame(step);
  }

  function bindRange(input, onInput) {
    var bubble = null, wrap = input.parentNode;
    if (wrap) {
      bubble = wrap.querySelector('.sim-range-val');
      if (!bubble) {
        bubble = document.createElement('output');
        bubble.className = 'sim-range-val';
        if (input.id) bubble.setAttribute('for', input.id);
        wrap.appendChild(bubble);
      }
    }
    function get() { return parseFloat(input.value); }
    function sync() {
      var v = get();
      var min = parseFloat(input.min || 0), max = parseFloat(input.max || 100);
      var unit = input.getAttribute('data-unit');
      input.style.setProperty('--fill', (max > min ? ((v - min) / (max - min)) * 100 : 0) + '%');
      if (bubble) bubble.textContent = unit ? v + ' ' + unit : String(v);
      if (onInput) onInput(v, input);
    }
    input.addEventListener('input', sync);
    sync();
    return get;
  }

  function svg(tag, attrs, children) {
    var el = document.createElementNS(NS, tag);
    if (attrs) {
      for (var k in attrs) {
        if (Object.prototype.hasOwnProperty.call(attrs, k) && attrs[k] != null) el.setAttribute(k, attrs[k]);
      }
    }
    if (children != null) {
      (Array.isArray(children) ? children : [children]).forEach(function (c) {
        if (c == null) return;
        el.appendChild(typeof c === 'string' ? document.createTextNode(c) : c);
      });
    }
    return el;
  }

  var registry = {}, started = {}, io = null;

  function findRoot(id) {
    return document.querySelector('.sim[data-sim="' + id + '"]') || document.getElementById(id);
  }

  function start(id) {
    if (started[id] || !registry[id]) return;
    var root = findRoot(id);
    if (!root) return;
    started[id] = true;
    if (io) io.unobserve(root);
    var note = root.querySelector('.sim-note');
    if (note) note.hidden = true;
    registry[id](root);
  }

  function registerSim(id, initFn) {
    registry[id] = initFn;
    var root = findRoot(id);
    if (!root) return;
    if (io) io.observe(root); else start(id);
  }

  if ('IntersectionObserver' in window) {
    io = new IntersectionObserver(function (entries) {
      entries.forEach(function (e) {
        if (!e.isIntersecting) return;
        var id = e.target.getAttribute('data-sim');
        if (id) start(id);
      });
    }, { threshold: 0.2 });
  }

  window.IbSim = {
    registerSim: registerSim, fmtEUR: fmtEUR, fmtInt: fmtInt, fmtH: fmtH,
    animateCount: animateCount, bindRange: bindRange, svg: svg,
    prefersReducedMotion: reduced
  };
})();

/* ═══ Simulateur 1 : ROI Calculator ═══ */
(function () {
  'use strict';
  var S = window.IbSim;
  if (!S) return;

  // Hypothèses de calcul : estimations paramétrables, jamais des faits garantis.
  var PART_ENGAGEMENTS = 0.15;   // part des messages qui contiennent un engagement (relance, promesse, échéance)
  var MIN_PAR_ENGAGEMENT = 2;    // minutes de suivi manuel évitées par engagement sécurisé
  var JOURS_OUVRES = 21;         // jours ouvrés par mois
  var TAUX_OUBLI_EVITE = 0.70;   // part des oublis évités grâce au suivi automatique

  var CHAMPS = [
    { id: 'boites', label: 'Boîtes mail suivies', min: 1, max: 20, val: 3, step: 1, unit: '' },
    { id: 'msgJour', label: 'Messages par jour et par boîte', min: 10, max: 300, val: 80, step: 5, unit: '' },
    { id: 'tauxRetard', label: 'Part de réponses en retard', min: 0, max: 50, val: 15, step: 1, unit: '%' },
    { id: 'coutHoraire', label: 'Coût horaire', min: 20, max: 150, val: 50, step: 5, unit: '€/h' },
    { id: 'coutOubli', label: 'Coût moyen d\'un oubli', min: 50, max: 5000, val: 400, step: 50, unit: '€' },
    { id: 'freqOubli', label: 'Oublis par mois', min: 0, max: 20, val: 4, step: 1, unit: '/mois' }
  ];

  function fin(x) { return isFinite(x) ? x : 0; }

  function calc(v) {
    var engagements = fin(v.boites * v.msgJour * PART_ENGAGEMENTS * JOURS_OUVRES);
    var temps = engagements * MIN_PAR_ENGAGEMENT / 60;
    var valeurTemps = temps * 12 * v.coutHoraire;
    var oublisEvites = v.freqOubli * 12 * v.coutOubli * TAUX_OUBLI_EVITE;
    var economies = valeurTemps + oublisEvites;
    var outil = v.coutSolution > 0 ? v.coutSolution * 12 : 0;
    var hasCost = outil > 0;
    return {
      engagements: engagements, temps: fin(temps), oublisEvites: fin(oublisEvites), economies: fin(economies),
      hasCost: hasCost, outil: outil,
      roi: hasCost ? (economies - outil) / outil * 100 : null,
      payback: hasCost && economies > 0 ? outil / (economies / 365) : null,
      sans: valeurTemps + v.freqOubli * 12 * v.coutOubli // coût annuel subi (temps perdu + oublis)
    };
  }

  function el(tag, cls, text) {
    var e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text != null) e.textContent = text;
    return e;
  }

  function init(root) {
    var corps = root.querySelector('.sim-corps');
    if (!corps) return;
    var vals = {}, ready = false;

    corps.appendChild(el('p', 'sim-etiquette', 'Estimation paramétrable, non garantie'));

    // Curseurs
    CHAMPS.forEach(function (c) {
      var champ = el('div', 'sim-champ');
      var lab = el('label', null, c.label);
      lab.setAttribute('for', 'sim1-' + c.id);
      var ligne = el('div', 'sim-range-ligne');
      var inp = document.createElement('input');
      inp.type = 'range'; inp.className = 'sim-range'; inp.id = 'sim1-' + c.id;
      inp.min = c.min; inp.max = c.max; inp.step = c.step; inp.value = c.val;
      if (c.unit) inp.setAttribute('data-unit', c.unit);
      ligne.appendChild(inp);
      champ.appendChild(lab); champ.appendChild(ligne);
      corps.appendChild(champ);
      vals[c.id] = c.val;
      S.bindRange(inp, function (v) { vals[c.id] = fin(v); if (ready) update(false); });
    });

    // Coût de la solution : champ éditable, tarif à confirmer (vide par défaut)
    var cs = el('div', 'sim-champ');
    var csLab = el('label', null, 'Coût de la solution (€/mois), à confirmer');
    csLab.setAttribute('for', 'sim1-coutSolution');
    var csInp = document.createElement('input');
    csInp.type = 'number'; csInp.id = 'sim1-coutSolution'; csInp.className = 'sim-input';
    csInp.min = '0'; csInp.step = '1'; csInp.inputMode = 'decimal';
    csInp.placeholder = 'Tarif à confirmer';
    csInp.setAttribute('aria-describedby', 'sim1-cs-aide');
    var csAide = el('small', 'sim-aide', 'Aucun tarif n\'est présupposé : saisissez un montant pour obtenir le ROI et le délai de retour.');
    csAide.id = 'sim1-cs-aide';
    cs.appendChild(csLab); cs.appendChild(csInp); cs.appendChild(csAide);
    corps.appendChild(cs);
    csInp.addEventListener('input', function () {
      var n = parseFloat(csInp.value);
      vals.coutSolution = isFinite(n) && n > 0 ? n : 0;
      if (ready) update(false);
    });
    vals.coutSolution = 0;

    // Sorties
    var sorties = el('div', 'sim-sorties');
    sorties.setAttribute('aria-live', 'polite');
    var DEFS = [
      { k: 'temps', label: 'Temps gagné (h/mois)', fmt: S.fmtH },
      { k: 'engagements', label: 'Engagements sécurisés (/mois)', fmt: S.fmtInt },
      { k: 'oublisEvites', label: 'Oublis évités (€/an)', fmt: S.fmtEUR },
      { k: 'economies', label: 'Économies (€/an)', fmt: S.fmtEUR, cls: 'sim-out-ok' },
      { k: 'roi', label: 'ROI', fmt: function (n) { return S.fmtInt(n) + ' %'; }, cls: 'sim-out-ok' },
      { k: 'payback', label: 'Retour en (jours)', fmt: S.fmtInt }
    ];
    var outs = {};
    DEFS.forEach(function (d) {
      var box = el('div', 'sim-out' + (d.cls ? ' ' + d.cls : ''));
      var strong = el('strong', null, '—');
      box.appendChild(strong); box.appendChild(el('span', null, d.label));
      sorties.appendChild(box);
      outs[d.k] = strong;
    });
    corps.appendChild(sorties);

    // Graphes
    var chartA = el('div', 'sim-chart');
    var chartB = el('div', 'sim-chart');
    corps.appendChild(chartA); corps.appendChild(chartB);

    // Formule
    var det = document.createElement('details');
    det.className = 'sim-formule';
    det.appendChild(el('summary', null, 'Voir le calcul'));
    det.appendChild(el('p', 'sim-formule-texte',
      'Engagements/mois = boîtes × messages/jour × ' + PART_ENGAGEMENTS + ' × ' + JOURS_OUVRES + ' jours. ' +
      'Temps gagné = engagements × ' + MIN_PAR_ENGAGEMENT + ' min ÷ 60. ' +
      'Valeur du temps/an = temps gagné × 12 × coût horaire. ' +
      'Oublis évités/an = oublis par mois × 12 × coût d\'un oubli × ' + TAUX_OUBLI_EVITE + '. ' +
      'Économies/an = valeur du temps + oublis évités. ' +
      'ROI = (économies − coût outil annuel) ÷ coût outil annuel, et retour = coût outil ÷ (économies ÷ 365), ' +
      'calculés seulement si vous saisissez un coût de solution. ' +
      'Les hypothèses (' + PART_ENGAGEMENTS + ', ' + MIN_PAR_ENGAGEMENT + ' min, ' + TAUX_OUBLI_EVITE + ') sont des estimations, pas des garanties.'));
    corps.appendChild(det);

    function drawBars(r) {
      var W = 320, H = 190, base = 150, top = 30, h = base - top;
      var avec = Math.max(0, r.sans - r.economies + r.outil);
      var max = Math.max(r.sans, avec, 1);
      var items = [
        { lab: 'Sans Ibalya', v: r.sans, cls: 'sim-bar-sans', x: 50 },
        { lab: 'Avec', v: avec, cls: 'sim-bar-avec', x: 190 }
      ];
      var kids = [S.svg('line', { x1: 20, x2: 300, y1: base, y2: base, 'class': 'sim-axe' })];
      items.forEach(function (it) {
        var bh = Math.max(2, it.v / max * h);
        kids.push(S.svg('rect', { x: it.x, y: base - bh, width: 80, height: bh, rx: 4, 'class': it.cls }));
        kids.push(S.svg('text', { x: it.x + 40, y: base - bh - 8, 'text-anchor': 'middle', 'class': 'sim-val' }, S.fmtEUR(it.v)));
        kids.push(S.svg('text', { x: it.x + 40, y: base + 20, 'text-anchor': 'middle', 'class': 'sim-lib' }, it.lab));
      });
      chartA.replaceChildren(S.svg('svg', { viewBox: '0 0 ' + W + ' ' + H, role: 'img',
        'aria-label': 'Coût annuel estimé sans Ibalya : ' + S.fmtEUR(r.sans) + ', avec : ' + S.fmtEUR(avec) }, kids));
    }

    function drawLine(r) {
      var W = 320, H = 190, x0 = 40, x1 = 305, yb = 150, yt = 35;
      var mensuel = (r.economies - r.outil) / 12, pts = [], i;
      for (i = 0; i <= 12; i++) pts.push(mensuel * i);
      var lo = Math.min(0, pts[12]), hi = Math.max(0, pts[12]);
      if (hi === lo) hi = lo + 1;
      function X(m) { return x0 + (x1 - x0) * m / 12; }
      function Y(v) { return yb - (v - lo) / (hi - lo) * (yb - yt); }
      var d = pts.map(function (v, m) { return (m ? 'L' : 'M') + X(m).toFixed(1) + ' ' + Y(v).toFixed(1); }).join(' ');
      var kids = [
        S.svg('line', { x1: x0, x2: x1, y1: Y(0), y2: Y(0), 'class': 'sim-axe' }),
        S.svg('path', { d: d, 'class': 'sim-ligne', fill: 'none' }),
        S.svg('circle', { cx: X(12), cy: Y(pts[12]), r: 4, 'class': 'sim-point' }),
        S.svg('text', { x: x1, y: Y(pts[12]) - 10, 'text-anchor': 'end', 'class': 'sim-val' }, S.fmtEUR(pts[12])),
        S.svg('text', { x: x0, y: yb + 22, 'text-anchor': 'start', 'class': 'sim-lib' }, 'Mois 0'),
        S.svg('text', { x: x1, y: yb + 22, 'text-anchor': 'end', 'class': 'sim-lib' }, 'Mois 12'),
        S.svg('text', { x: x0, y: 14, 'text-anchor': 'start', 'class': 'sim-lib' },
          r.hasCost ? 'Cumul net sur 12 mois (outil déduit)' : 'Cumul brut sur 12 mois')
      ];
      chartB.replaceChildren(S.svg('svg', { viewBox: '0 0 ' + W + ' ' + H, role: 'img',
        'aria-label': 'Cumul estimé sur 12 mois : ' + S.fmtEUR(pts[12]) }, kids));
    }

    function update(animate) {
      var r = calc(vals);
      DEFS.forEach(function (d) {
        var node = outs[d.k], v = r[d.k];
        if (node._ibRaf) { cancelAnimationFrame(node._ibRaf); node._ibRaf = 0; }
        if (v == null || !isFinite(v)) { node.textContent = '—'; return; }
        // Animation réservée à la première révélation ; ensuite écriture directe (pas de scintillement).
        if (animate) S.animateCount(node, 0, v, d.fmt);
        else node.textContent = d.fmt(v);
      });
      drawBars(r); drawLine(r);
    }

    ready = true;
    update(true);
  }

  S.registerSim('roi', init);
})();
