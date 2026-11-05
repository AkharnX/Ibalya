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
    // Plus la part de réponses en retard est élevée, plus le relancement manuel épargné est important.
    var temps = engagements * MIN_PAR_ENGAGEMENT / 60 * (1 + v.tauxRetard / 100);
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
      'Temps gagné = engagements × ' + MIN_PAR_ENGAGEMENT + ' min ÷ 60 × (1 + part en retard ÷ 100) : ' +
      'plus d\'engagements en retard, plus de temps de relance manuel épargné. ' +
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

/* ═══ Simulateur 2 : Coût de l'oubli (jauge) ═══ */
(function () {
  'use strict';
  var S = window.IbSim;
  if (!S) return;

  // Hypothèses : estimations paramétrables, jamais des faits mesurés.
  var MOIS_PAR_AN = 12;        // conversion fréquence mensuelle -> annuelle
  var PLANCHER_JAUGE = 1000;     // € par an : début d'échelle (aiguille à gauche en dessous)
  var PLAFOND_JAUGE = 1000000;   // € par an : fin d'échelle (aiguille en butée au-delà)
  // Position de l'aiguille en échelle logarithmique (repère visuel uniquement ; les montants affichés restent linéaires).

  var CHAMPS = [
    { id: 'coutMoyenOubli', label: 'Coût moyen d\'un oubli', min: 50, max: 5000, val: 400, step: 50, unit: '€' },
    { id: 'frequenceMois', label: 'Oublis par mois', min: 0, max: 30, val: 4, step: 1, unit: '/mois' },
    { id: 'tauxTransfo', label: 'Taux de transformation perdu', min: 0, max: 100, val: 30, step: 5, unit: '%' }
  ];

  function fin(x) { return isFinite(x) ? x : 0; }
  function el(tag, cls, text) {
    var e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text != null) e.textContent = text;
    return e;
  }

  function calc(v) {
    var direct = fin(v.coutMoyenOubli * v.frequenceMois * MOIS_PAR_AN);
    var indirect = fin(direct * v.tauxTransfo / 100);
    return { direct: direct, indirect: indirect, total: direct + indirect };
  }

  function init(root) {
    var corps = root.querySelector('.sim-corps');
    if (!corps) return;
    var vals = {}, ready = false;

    corps.appendChild(el('p', 'sim-etiquette', 'Estimation paramétrable, non garantie'));

    CHAMPS.forEach(function (c) {
      var champ = el('div', 'sim-champ');
      var lab = el('label', null, c.label);
      lab.setAttribute('for', 'sim2-' + c.id);
      var ligne = el('div', 'sim-range-ligne');
      var inp = document.createElement('input');
      inp.type = 'range'; inp.className = 'sim-range'; inp.id = 'sim2-' + c.id;
      inp.min = c.min; inp.max = c.max; inp.step = c.step; inp.value = c.val;
      inp.setAttribute('data-unit', c.unit);
      ligne.appendChild(inp);
      champ.appendChild(lab); champ.appendChild(ligne);
      corps.appendChild(champ);
      vals[c.id] = c.val;
      S.bindRange(inp, function (v) { vals[c.id] = fin(v); if (ready) update(false); });
    });

    var sorties = el('div', 'sim-sorties');
    sorties.setAttribute('aria-live', 'polite');
    var DEFS = [
      { k: 'direct', label: 'Coût direct (€/an)' },
      { k: 'indirect', label: 'Coût indirect, opportunités perdues (€/an)', cls: 'sim-out-warn' },
      { k: 'total', label: 'Coût total estimé (€/an)', cls: 'sim-out-risk' }
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

    // Jauge demi-cercle : 3 tronçons (vert, ambre, brique) + aiguille.
    var W = 320, H = 190, CX = 160, CY = 150, R = 110, NEEDLE = 96;
    function pt(frac, r) { // frac 0 = gauche, 1 = droite, sur le demi-cercle supérieur
      var a = Math.PI * (1 - frac);
      return (CX + r * Math.cos(a)).toFixed(2) + ' ' + (CY - r * Math.sin(a)).toFixed(2);
    }
    function arc(f0, f1, cls) {
      return S.svg('path', { d: 'M' + pt(f0, R) + ' A' + R + ' ' + R + ' 0 0 1 ' + pt(f1, R), fill: 'none', 'class': 'sim-jauge-arc ' + cls });
    }
    var needle = S.svg('g', { 'class': 'sim-aiguille' }, [
      S.svg('line', { x1: CX, y1: CY, x2: CX - NEEDLE, y2: CY, 'class': 'sim-aiguille-trait' }),
      S.svg('circle', { cx: CX, cy: CY, r: 7, 'class': 'sim-aiguille-pivot' })
    ]);
    var valTxt = S.svg('text', { x: CX, y: CY + 30, 'text-anchor': 'middle', 'class': 'sim-val' }, '');
    var gauge = S.svg('svg', { viewBox: '0 0 ' + W + ' ' + H, role: 'img', 'aria-label': '' }, [
      arc(0, 1 / 3, 'sim-jauge-ok'), arc(1 / 3, 2 / 3, 'sim-jauge-warn'), arc(2 / 3, 1, 'sim-jauge-risk'),
      S.svg('text', { x: CX - R, y: CY + 20, 'text-anchor': 'middle', 'class': 'sim-lib' }, '≤ ' + S.fmtEUR(PLANCHER_JAUGE)),
      S.svg('text', { x: CX + R, y: CY + 20, 'text-anchor': 'middle', 'class': 'sim-lib' }, '1 M€+'),
      needle, valTxt
    ]);
    var chart = el('div', 'sim-chart');
    chart.appendChild(gauge);
    corps.appendChild(chart);

    var det = document.createElement('details');
    det.className = 'sim-formule';
    det.appendChild(el('summary', null, 'Voir le calcul'));
    det.appendChild(el('p', 'sim-formule-texte',
      'Coût direct/an = coût moyen d\'un oubli × oublis par mois × ' + MOIS_PAR_AN + ' mois. ' +
      'Coût indirect/an = coût direct × taux de transformation perdu ÷ 100 (opportunités qui ne se concrétisent pas). ' +
      'Total = direct + indirect. ' +
      'La position de l\'aiguille suit une échelle logarithmique entre ' + S.fmtEUR(PLANCHER_JAUGE) + ' et ' + S.fmtEUR(PLAFOND_JAUGE) + ' par an (butée aux extrémités) : ' +
      'elle sert à lire l\'ordre de grandeur, les montants affichés restent linéaires. Cette échelle est un repère de lecture, pas un seuil réel. Toutes les valeurs sont des estimations que vous réglez, pas des mesures.'));
    corps.appendChild(det);

    function update(animate) {
      var r = calc(vals);
      DEFS.forEach(function (d) {
        var node = outs[d.k], v = r[d.k];
        if (node._ibRaf) { cancelAnimationFrame(node._ibRaf); node._ibRaf = 0; }
        // Animation réservée à la première révélation ; ensuite écriture directe.
        if (animate) S.animateCount(node, 0, v, S.fmtEUR);
        else node.textContent = S.fmtEUR(v);
      });
      var frac = Math.max(0, Math.min(1, Math.log(Math.max(r.total, PLANCHER_JAUGE) / PLANCHER_JAUGE) / Math.log(PLAFOND_JAUGE / PLANCHER_JAUGE)));
      // La transition CSS est désactivée sous prefers-reduced-motion (voir style.css).
      needle.style.transform = 'rotate(' + (frac * 180).toFixed(1) + 'deg)';
      valTxt.textContent = S.fmtEUR(r.total) + ' / an';
      gauge.setAttribute('aria-label', 'Coût annuel estimé de l\'oubli : ' + S.fmtEUR(r.total) +
        ' (échelle logarithmique de ' + S.fmtEUR(PLANCHER_JAUGE) + ' à ' + S.fmtEUR(PLAFOND_JAUGE) + ')');
    }

    ready = true;
    update(true);
  }

  S.registerSim('oubli', init);
})();

/* ═══ Simulateur 3 : Temps perdu (heatmap calendaire) ═══ */
(function () {
  'use strict';
  var S = window.IbSim;
  if (!S) return;

  // Hypothèses : estimations paramétrables, jamais des faits mesurés.
  var JOURS_OUVRES = 230;      // jours travaillés par an
  var MIN_PAR_H = 60;          // minutes par heure
  var H_PAR_JOUR = 8;          // heures par jour ouvré (équivalence en jours)
  var COLS = 23, ROWS = 10;    // grille 23 x 10 = 230 cases = exactement les jours ouvrés (une case = un jour ouvré)
  var SEUIL_PLEIN = 120;       // minutes perdues par jour pour la teinte pleine (échelle fixe, pour que la grille réagisse)
  var AMPLITUDE = 0.45;        // variation illustrative autour de la moyenne

  // Répartition illustrative et DÉTERMINISTE : fonction fixe de l'indice de case (aucun hasard).
  // Normalisée pour que la moyenne sur les 230 jours vaille exactement 1 : la grille redistribue
  // le total calculé, elle n'invente aucun volume.
  var FACTEURS = (function () {
    var f = [], i, s = 0, m;
    for (i = 0; i < JOURS_OUVRES; i++) {
      var onde = Math.sin(i * 0.37) * 0.6 + Math.sin(i * 1.3 + 1) * 0.4;  // ondulation lente + rapide
      var semaine = [0.25, 0, -0.1, 0.05, -0.3][i % 5];                    // lundi chargé, vendredi plus calme
      f.push(Math.max(0.05, 1 + AMPLITUDE * onde + semaine));
      s += f[i];
    }
    m = s / JOURS_OUVRES;
    return f.map(function (x) { return x / m; });
  })();

  var CHAMPS = [
    { id: 'minRecherche', label: 'Durée d\'une recherche', min: 1, max: 10, val: 2, step: 1, unit: 'min' },
    { id: 'recherchesJour', label: 'Recherches par jour', min: 1, max: 40, val: 10, step: 1, unit: '/jour' },
    { id: 'coutHoraire', label: 'Coût horaire', min: 20, max: 150, val: 50, step: 5, unit: '€/h' }
  ];

  function fin(x) { return isFinite(x) ? x : 0; }
  function el(tag, cls, text) {
    var e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text != null) e.textContent = text;
    return e;
  }

  function calc(v) {
    var minParJour = fin(v.recherchesJour * v.minRecherche);
    var hParAn = fin(minParJour * JOURS_OUVRES / MIN_PAR_H);
    return { minParJour: minParJour, hParAn: hParAn, jours: fin(hParAn / H_PAR_JOUR), coutParAn: fin(hParAn * v.coutHoraire) };
  }

  function fmtJ(n) { return S.fmtH(n).replace(/\s*h$/, ' j'); }

  function init(root) {
    var corps = root.querySelector('.sim-corps');
    if (!corps) return;
    var vals = {}, ready = false;

    corps.appendChild(el('p', 'sim-etiquette', 'Estimation paramétrable, non garantie'));

    CHAMPS.forEach(function (c) {
      var champ = el('div', 'sim-champ');
      var lab = el('label', null, c.label);
      lab.setAttribute('for', 'sim3-' + c.id);
      var ligne = el('div', 'sim-range-ligne');
      var inp = document.createElement('input');
      inp.type = 'range'; inp.className = 'sim-range'; inp.id = 'sim3-' + c.id;
      inp.min = c.min; inp.max = c.max; inp.step = c.step; inp.value = c.val;
      inp.setAttribute('data-unit', c.unit);
      ligne.appendChild(inp);
      champ.appendChild(lab); champ.appendChild(ligne);
      corps.appendChild(champ);
      vals[c.id] = c.val;
      S.bindRange(inp, function (v) { vals[c.id] = fin(v); if (ready) update(false); });
    });

    var sorties = el('div', 'sim-sorties');
    sorties.setAttribute('aria-live', 'polite');
    var DEFS = [
      { k: 'hParAn', label: 'Heures perdues par an', fmt: S.fmtH },
      { k: 'jours', label: 'Jours ouvrés équivalents', fmt: fmtJ, cls: 'sim-out-warn' },
      { k: 'coutParAn', label: 'Coût estimé (€/an)', fmt: S.fmtEUR, cls: 'sim-out-risk' }
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

    // Heatmap : une case = un jour ouvré, rangées en colonnes de 12 (décorative, le résumé est dans aria-label).
    var CELL = 14, GAP = 3, PAD = 2, LEG = 22;
    var W = PAD * 2 + COLS * (CELL + GAP) - GAP, H = PAD * 2 + ROWS * (CELL + GAP) - GAP + LEG;
    var cells = [];
    var grid = S.svg('g', { 'aria-hidden': 'true' });
    for (var i = 0; i < JOURS_OUVRES; i++) {
      var r = S.svg('rect', {
        x: PAD + Math.floor(i / ROWS) * (CELL + GAP), y: PAD + (i % ROWS) * (CELL + GAP),
        width: CELL, height: CELL, rx: 3, 'class': 'sim-heat-case', 'fill-opacity': 0.1
      });
      cells.push(r); grid.appendChild(r);
    }
    var legY = H - 8;
    var legende = S.svg('g', { 'aria-hidden': 'true' }, [
      S.svg('text', { x: PAD, y: legY, 'class': 'sim-lib' }, '0 min'),
      S.svg('text', { x: W / 2, y: legY, 'text-anchor': 'middle', 'class': 'sim-lib' }, 'répartition illustrative'),
      S.svg('text', { x: W - PAD, y: legY, 'text-anchor': 'end', 'class': 'sim-lib' }, '≥ ' + SEUIL_PLEIN + ' min/jour')
    ]);
    var chartSvg = S.svg('svg', { viewBox: '0 0 ' + W + ' ' + H, role: 'img', 'aria-label': '' }, [grid, legende]);
    var chart = el('div', 'sim-chart');
    chart.appendChild(chartSvg);
    corps.appendChild(chart);
    corps.appendChild(el('p', 'sim-aide',
      'Chaque case est un jour ouvré (' + JOURS_OUVRES + ' par an). Plus la teinte est foncée, plus le temps perdu ce jour-là est élevé. ' +
      'La répartition entre les jours est illustrative : seule la moyenne est calculée.'));

    var det = document.createElement('details');
    det.className = 'sim-formule';
    det.appendChild(el('summary', null, 'Voir le calcul'));
    det.appendChild(el('p', 'sim-formule-texte',
      'Minutes perdues par jour = recherches par jour × durée d\'une recherche. ' +
      'Heures par an = minutes par jour × ' + JOURS_OUVRES + ' jours ouvrés ÷ ' + MIN_PAR_H + '. ' +
      'Jours équivalents = heures par an ÷ ' + H_PAR_JOUR + ' h. ' +
      'Coût par an = heures par an × coût horaire. ' +
      'La grille répartit ce total selon un motif fixe (ondulation et rythme de semaine), normalisé pour que la moyenne égale exactement les minutes par jour calculées. ' +
      'Ce motif est illustratif, pas une mesure. Toutes les valeurs sont des estimations que vous réglez.'));
    corps.appendChild(det);

    function update(animate) {
      var res = calc(vals);
      DEFS.forEach(function (d) {
        var node = outs[d.k], v = res[d.k];
        if (node._ibRaf) { cancelAnimationFrame(node._ibRaf); node._ibRaf = 0; }
        // Animation réservée à la première révélation ; ensuite écriture directe (pas de scintillement).
        if (animate) S.animateCount(node, 0, v, d.fmt);
        else node.textContent = d.fmt(v);
      });
      for (var k = 0; k < cells.length; k++) {
        var t = Math.min(1, fin(res.minParJour * FACTEURS[k]) / SEUIL_PLEIN);
        cells[k].setAttribute('fill-opacity', (0.1 + 0.9 * t).toFixed(3));
        if (animate && !S.prefersReducedMotion) {
          cells[k].style.animationDelay = (Math.floor(k / ROWS) * 35) + 'ms';
          cells[k].classList.add('sim-heat-reveal');
        }
      }
      chartSvg.setAttribute('aria-label', 'Temps perdu en recherches : ' + S.fmtH(res.hParAn) + ' par an, soit ' +
        S.fmtEUR(res.coutParAn) + ' (estimation). Grille de ' + JOURS_OUVRES + ' jours ouvrés, répartition illustrative.');
    }

    ready = true;
    update(true);
  }

  S.registerSim('temps', init);
})();

/* ═══ Simulateur 4 : Exposition au risque (heatmap 7 x 24) ═══ */
(function () {
  'use strict';
  var S = window.IbSim;
  if (!S) return;

  // Modèle illustratif : estimations paramétrables, jamais des faits mesurés.
  var P_RETARD = 0.6, P_SILENCE = 0.4;   // poids des deux taux dans le score
  var NORM = 30;                          // pondéré (en points de %) donnant 100 à volume maximal
  var VOL_MIN = 0.4, VOL_MAX = 200;       // effet du volume : 0,4 à 1 (échelle logarithmique)
  var SEUIL_MOYEN = 33, SEUIL_ELEVE = 66; // libellés : < 33 faible, < 66 moyen, sinon élevé
  var JOURS = ['Lun', 'Mar', 'Mer', 'Jeu', 'Ven', 'Sam', 'Dim'];

  // Profil déterministe jour x heure (aucun hasard) : plus chaud en journée et en semaine.
  // Normalisé pour que la moyenne sur les 168 cases vaille 1.
  var PROFIL = (function () {
    var jf = [1.0, 1.1, 1.1, 1.05, 0.95, 0.25, 0.12], p = [], s = 0, d, h, m;
    for (d = 0; d < 7; d++) {
      for (h = 0; h < 24; h++) {
        var jour = Math.exp(-Math.pow((h - 13) / 4.5, 2) / 2);              // pic en milieu de journée
        var pause = (h === 12 || h === 13) ? 0.85 : 1;                      // léger creux à midi
        var v = jf[d] * (0.04 + jour * pause);
        p.push(v); s += v;
      }
    }
    m = s / p.length;
    return p.map(function (x) { return x / m; });
  })();

  var CHAMPS = [
    { id: 'engagements', label: 'Engagements suivis', min: 1, max: 200, val: 40, step: 1, unit: '' },
    { id: 'tauxRetard', label: 'Part en retard', min: 0, max: 60, val: 20, step: 1, unit: '%' },
    { id: 'tauxSilence', label: 'Part sans réponse', min: 0, max: 60, val: 15, step: 1, unit: '%' }
  ];

  function fin(x) { return isFinite(x) ? x : 0; }
  function el(tag, cls, text) {
    var e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text != null) e.textContent = text;
    return e;
  }
  function clamp(x, a, b) { return Math.min(b, Math.max(a, x)); }

  function calc(v) {
    var vol = VOL_MIN + (1 - VOL_MIN) * clamp(Math.log(v.engagements) / Math.log(VOL_MAX), 0, 1);
    var pond = P_RETARD * v.tauxRetard + P_SILENCE * v.tauxSilence;
    var score = clamp(fin(100 * (pond / NORM) * vol), 0, 100);
    var aRisque = clamp(fin(v.engagements * (v.tauxRetard + v.tauxSilence) / 100), 0, v.engagements);
    var lib = score < SEUIL_MOYEN ? 'faible' : (score < SEUIL_ELEVE ? 'moyen' : 'élevé');
    var cats = [
      { nom: 'Échéance à risque', n: v.engagements * v.tauxRetard / 100, ordre: 0 },
      { nom: 'Silence anormal', n: v.engagements * v.tauxSilence / 100, ordre: 1 },
      { nom: 'Promesse menacée', n: v.engagements * Math.min(v.tauxRetard, v.tauxSilence) / 100, ordre: 2 }
    ].sort(function (a, b) { return (b.n - a.n) || (a.ordre - b.ordre); });
    return { score: score, lib: lib, aRisque: aRisque, top: cats };
  }

  function init(root) {
    var corps = root.querySelector('.sim-corps');
    if (!corps) return;
    var vals = {}, ready = false;

    corps.appendChild(el('p', 'sim-etiquette', 'Estimation illustrative, non garantie'));

    CHAMPS.forEach(function (c) {
      var champ = el('div', 'sim-champ');
      var lab = el('label', null, c.label);
      lab.setAttribute('for', 'sim4-' + c.id);
      var ligne = el('div', 'sim-range-ligne');
      var inp = document.createElement('input');
      inp.type = 'range'; inp.className = 'sim-range'; inp.id = 'sim4-' + c.id;
      inp.min = c.min; inp.max = c.max; inp.step = c.step; inp.value = c.val;
      if (c.unit) inp.setAttribute('data-unit', c.unit);
      ligne.appendChild(inp);
      champ.appendChild(lab); champ.appendChild(ligne);
      corps.appendChild(champ);
      vals[c.id] = c.val;
      S.bindRange(inp, function (v) { vals[c.id] = fin(v); if (ready) update(false); });
    });

    var sorties = el('div', 'sim-sorties');
    sorties.setAttribute('aria-live', 'polite');
    var boxScore = el('div', 'sim-out');
    var sScore = el('strong', null, '—');
    boxScore.appendChild(sScore); boxScore.appendChild(el('span', null, 'Score de risque (0 à 100)'));
    var boxNiveau = el('div', 'sim-out');
    var sNiveau = el('strong', null, '—');
    boxNiveau.appendChild(sNiveau); boxNiveau.appendChild(el('span', null, 'Niveau'));
    var boxRisque = el('div', 'sim-out sim-out-warn');
    var sRisque = el('strong', null, '—');
    boxRisque.appendChild(sRisque); boxRisque.appendChild(el('span', null, 'Engagements à risque'));
    sorties.appendChild(boxScore); sorties.appendChild(boxNiveau); sorties.appendChild(boxRisque);
    corps.appendChild(sorties);

    var topBox = el('div', 'sim-top');
    topBox.appendChild(el('p', 'sim-top-titre', 'Top 3 des risques (catégories dérivées de vos réglages)'));
    var ul = el('ul', 'sim-top-liste');
    var lis = [];
    for (var q = 0; q < 3; q++) {
      var li = el('li');
      var nom = el('span', 'sim-top-nom'), nb = el('span', 'sim-top-n');
      li.appendChild(nom); li.appendChild(nb); ul.appendChild(li);
      lis.push({ nom: nom, nb: nb });
    }
    topBox.appendChild(ul);
    corps.appendChild(topBox);

    // Heatmap 7 x 24 (décorative, le résumé est dans aria-label).
    var CELL = 11, GAP = 2, PADL = 28, PADT = 14, LEG = 22;
    var W = PADL + 24 * (CELL + GAP) - GAP + 2, H = PADT + 7 * (CELL + GAP) - GAP + LEG;
    var cells = [];
    var grid = S.svg('g', { 'aria-hidden': 'true' });
    var d, h;
    for (d = 0; d < 7; d++) {
      grid.appendChild(S.svg('text', { x: 0, y: PADT + d * (CELL + GAP) + CELL - 2, 'class': 'sim-lib' }, JOURS[d]));
      for (h = 0; h < 24; h++) {
        var r = S.svg('rect', {
          x: PADL + h * (CELL + GAP), y: PADT + d * (CELL + GAP),
          width: CELL, height: CELL, rx: 2, 'class': 'sim-heat-case', 'fill-opacity': 0.1
        });
        cells.push(r); grid.appendChild(r);
      }
    }
    [0, 6, 12, 18, 23].forEach(function (hh) {
      var ax = PADL + hh * (CELL + GAP) + CELL / 2;
      grid.appendChild(S.svg('text', hh === 23 ? { x: ax + CELL / 2, y: 9, 'text-anchor': 'end', 'class': 'sim-lib' } : { x: ax, y: 9, 'text-anchor': 'middle', 'class': 'sim-lib' }, hh + ' h'));
    });
    var legY = H - 8;
    var legende = S.svg('g', { 'aria-hidden': 'true' }, [
      S.svg('text', { x: PADL, y: legY, 'class': 'sim-lib' }, 'risque faible'),
      S.svg('text', { x: W / 2 + 14, y: legY, 'text-anchor': 'middle', 'class': 'sim-lib' }, 'illustratif'),
      S.svg('text', { x: W - 4, y: legY, 'text-anchor': 'end', 'class': 'sim-lib' }, 'risque élevé')
    ]);
    var chartSvg = S.svg('svg', { viewBox: '0 0 ' + W + ' ' + H, role: 'img', 'aria-label': '' }, [grid, legende]);
    var chart = el('div', 'sim-chart');
    chart.appendChild(chartSvg);
    corps.appendChild(chart);
    corps.appendChild(el('p', 'sim-aide',
      'Chaque case est une heure d\'un jour de la semaine. Le niveau global de la grille suit le score ; ' +
      'la répartition entre jours et heures est illustrative (plus chaud en journée et en semaine), pas une mesure de vos données.'));

    var det = document.createElement('details');
    det.className = 'sim-formule';
    det.appendChild(el('summary', null, 'Voir le calcul'));
    det.appendChild(el('p', 'sim-formule-texte',
      'Score = 100 × (' + P_RETARD + ' × part en retard + ' + P_SILENCE + ' × part sans réponse) ÷ ' + NORM +
      ' × effet du volume, borné entre 0 et 100. L\'effet du volume croît de ' + VOL_MIN + ' à 1 selon le logarithme du nombre d\'engagements (1 à ' + VOL_MAX + '). ' +
      'Niveau : faible sous ' + SEUIL_MOYEN + ', moyen sous ' + SEUIL_ELEVE + ', élevé au-delà. ' +
      'Engagements à risque = engagements × (part en retard + part sans réponse) ÷ 100, plafonné au total. ' +
      'Top 3 : trois catégories génériques classées par effectif estimé (retard, silence, et le recoupement des deux pour « promesse menacée »). ' +
      'Aucun client, montant ou date n\'est supposé. Modèle illustratif, valeurs estimées.'));
    corps.appendChild(det);

    function update(animate) {
      var res = calc(vals);
      var cls = res.lib === 'faible' ? 'sim-out-ok' : (res.lib === 'moyen' ? 'sim-out-warn' : 'sim-out-risk');
      boxScore.className = 'sim-out ' + cls; boxNiveau.className = 'sim-out ' + cls;
      [[sScore, res.score, S.fmtInt], [sRisque, res.aRisque, S.fmtInt]].forEach(function (a) {
        var node = a[0];
        if (node._ibRaf) { cancelAnimationFrame(node._ibRaf); node._ibRaf = 0; }
        if (animate) S.animateCount(node, 0, a[1], a[2]);
        else node.textContent = a[2](a[1]);
      });
      sNiveau.textContent = res.lib.charAt(0).toUpperCase() + res.lib.slice(1);
      res.top.forEach(function (c, i) {
        lis[i].nom.textContent = (i + 1) + '. ' + c.nom + ' ';
        lis[i].nb.textContent = '≈ ' + S.fmtInt(c.n) + ' engagement' + (Math.round(c.n) > 1 ? 's' : '');
      });
      for (var k = 0; k < cells.length; k++) {
        var t = clamp(res.score / 100 * PROFIL[k], 0, 1);
        cells[k].style.fill = t < 0.33 ? 'var(--ok)' : (t < 0.66 ? 'var(--warn)' : 'var(--risk)');
        cells[k].setAttribute('fill-opacity', (0.12 + 0.88 * t).toFixed(3));
        if (animate && !S.prefersReducedMotion) {
          cells[k].style.animationDelay = (Math.floor(k / 24) * 60) + 'ms';
          cells[k].classList.add('sim-heat-reveal');
        }
      }
      chartSvg.setAttribute('aria-label', 'Exposition au risque : score ' + S.fmtInt(res.score) + ' sur 100 (' + res.lib +
        '), environ ' + S.fmtInt(res.aRisque) + ' engagements à risque (estimation). Grille de 7 jours par 24 heures, répartition illustrative.');
    }

    ready = true;
    update(true);
  }

  S.registerSim('risque', init);
})();
