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
    annonceCalme(root, id);
    scinder(root);
  }

  // Mise en page du panneau : entrées à gauche, résultats à droite. Les simulateurs construisent
  // leur contenu à plat dans .sim-corps ; on le répartit ensuite dans deux colonnes. Un .sim-aide
  // suit la colonne de l'élément qui le précède (il commente un curseur ou un graphique).
  var RESULTAT = /(^|\s)(sim-sorties|sim-chart|sim-formule|sim-top|sim-sr)(\s|$)/;
  function scinder(root) {
    var corps = root.querySelector('.sim-corps');
    if (!corps || corps.querySelector('.sim-entrees')) return;
    var entrees = document.createElement('div'); entrees.className = 'sim-entrees';
    var resultats = document.createElement('div'); resultats.className = 'sim-resultats';
    var cible = entrees;
    Array.prototype.slice.call(corps.children).forEach(function (c) {
      var cl = typeof c.className === 'string' ? c.className : '';
      if (!/(^|\s)sim-aide(\s|$)/.test(cl)) cible = RESULTAT.test(cl) ? resultats : entrees;
      cible.appendChild(c);
    });
    corps.appendChild(entrees);
    corps.appendChild(resultats);
  }

  // Annonces lecteur d'écran calmes et identiques pour tous les simulateurs : les sorties ne sont pas
  // des zones live (sinon chaque pas de curseur et chaque image de compteur seraient lus) ; un seul
  // résumé est posé dans une zone status, après 900 ms sans changement. Sim 5 a son propre statut.
  function annonceCalme(root, id) {
    if (id === 'validation' || !window.MutationObserver) return;
    var sorties = root.querySelector('.sim-sorties');
    if (!sorties) return;
    var statut = document.createElement('p');
    statut.className = 'sim-sr';
    statut.setAttribute('role', 'status');
    statut.setAttribute('aria-atomic', 'true');
    sorties.parentNode.insertBefore(statut, sorties.nextSibling);
    var timer = 0, touche = false;
    function resume() {
      return Array.prototype.map.call(sorties.children, function (b) {
        return Array.prototype.map.call(b.children, function (c) { return c.textContent; }).join(' ');
      }).join(', ');
    }
    var derniere = resume();
    ['input', 'change', 'click', 'keydown'].forEach(function (ev) {
      root.addEventListener(ev, function () { touche = true; }, true);
    });
    new MutationObserver(function () {
      clearTimeout(timer);
      timer = setTimeout(function () {
        var r = resume();
        if (r === derniere) return;
        // Tant que le visiteur n'a rien touché, c'est la révélation initiale : mémorisée, jamais annoncée.
        if (touche) statut.textContent = r;
        derniere = r;
      }, 900);
    }).observe(sorties, { childList: true, characterData: true, subtree: true });
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

    corps.appendChild(el('p', 'sim-etiquette', 'Estimation à partir de vos réglages'));

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
    var DEFS = [
      { k: 'temps', label: 'Temps gagné (h/mois)', fmt: S.fmtH },
      { k: 'engagements', label: 'Engagements sécurisés (/mois)', fmt: S.fmtInt },
      { k: 'oublisEvites', label: 'Oublis évités (€/an)', fmt: S.fmtEUR },
      { k: 'economies', label: 'Économies (€/an)', fmt: S.fmtEUR, cls: 'sim-out-ok' },
      { k: 'roi', label: 'ROI', fmt: function (n) { return n > 10000 ? '> 10 000 %' : S.fmtInt(n) + ' %'; }, cls: 'sim-out-ok' },
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

    corps.appendChild(el('p', 'sim-etiquette', 'Estimation à partir de vos réglages'));

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
      S.svg('text', { x: CX + R, y: CY + 20, 'text-anchor': 'middle', 'class': 'sim-lib' }, (PLAFOND_JAUGE / 1000000) + ' M€+'),
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

/* ═══ Simulateur 3 : Temps perdu (cumul mois par mois) ═══ */
(function () {
  'use strict';
  var S = window.IbSim;
  if (!S) return;

  // Hypothèses : estimations paramétrables, jamais des faits mesurés.
  var JOURS_OUVRES = 230;      // jours travaillés par an
  var MIN_PAR_H = 60;          // minutes par heure
  var H_PAR_JOUR = 8;          // heures par jour ouvré (équivalence en jours)
  var MOIS = 12;

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

    corps.appendChild(el('p', 'sim-etiquette', 'Estimation à partir de vos réglages'));

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

    // Cumul des heures perdues, mois par mois : dérivé du calcul, aucune répartition supposée.
    var W = 320, H = 160, PADL = 6, PADR = 6, PADT = 24, PADB = 22, GAP = 6;
    var barW = (W - PADL - PADR - GAP * (MOIS - 1)) / MOIS;
    var bars = [], g = S.svg('g', { 'aria-hidden': 'true' });
    for (var i = 0; i < MOIS; i++) {
      var r = S.svg('rect', { x: (PADL + i * (barW + GAP)).toFixed(1), y: H - PADB, width: barW.toFixed(1), height: 0, rx: 3, 'class': 'sim-barre-mois' });
      bars.push(r); g.appendChild(r);
    }
    var axe = S.svg('line', { x1: PADL, y1: H - PADB + 0.5, x2: W - PADR, y2: H - PADB + 0.5, 'class': 'sim-axe' });
    var lib1 = S.svg('text', { x: PADL, y: H - 6, 'class': 'sim-lib' }, 'Mois 1');
    var lib12 = S.svg('text', { x: W - PADR, y: H - 6, 'text-anchor': 'end', 'class': 'sim-lib' }, 'Mois 12');
    var valTop = S.svg('text', { x: W - PADR, y: 12, 'text-anchor': 'end', 'class': 'sim-val' }, '');
    var titre = S.svg('text', { x: PADL, y: 12, 'class': 'sim-lib' }, 'Cumul des heures perdues');
    var chartSvg = S.svg('svg', { viewBox: '0 0 ' + W + ' ' + H, role: 'img', 'aria-label': '' }, [titre, valTop, g, axe, lib1, lib12]);
    var chart = el('div', 'sim-chart');
    chart.appendChild(chartSvg);
    corps.appendChild(chart);

    var det = document.createElement('details');
    det.className = 'sim-formule';
    det.appendChild(el('summary', null, 'Voir le calcul'));
    det.appendChild(el('p', 'sim-formule-texte',
      'Minutes perdues par jour = recherches par jour × durée d\'une recherche. ' +
      'Heures par an = minutes par jour × ' + JOURS_OUVRES + ' jours ouvrés ÷ ' + MIN_PAR_H + '. ' +
      'Jours équivalents = heures par an ÷ ' + H_PAR_JOUR + ' h. ' +
      'Coût par an = heures par an × coût horaire. ' +
      'Le graphique cumule ce total par douzièmes, mois après mois. Toutes les valeurs sont des estimations que vous réglez.'));
    corps.appendChild(det);

    var first = true;
    function update(animate) {
      var res = calc(vals);
      DEFS.forEach(function (d) {
        var node = outs[d.k], v = res[d.k];
        if (node._ibRaf) { cancelAnimationFrame(node._ibRaf); node._ibRaf = 0; }
        if (animate) S.animateCount(node, 0, v, d.fmt);
        else node.textContent = d.fmt(v);
      });
      var hmax = H - PADB - PADT;
      for (var k = 0; k < MOIS; k++) {
        var h = res.hParAn > 0 ? hmax * (k + 1) / MOIS : 0;
        bars[k].setAttribute('y', (H - PADB - h).toFixed(1));
        bars[k].setAttribute('height', h.toFixed(1));
        if (animate && first && !S.prefersReducedMotion) {
          bars[k].style.animationDelay = (k * 40) + 'ms';
          bars[k].classList.add('sim-heat-reveal');
        }
      }
      first = false;
      valTop.textContent = S.fmtH(res.hParAn) + ' à 12 mois';
      chartSvg.setAttribute('aria-label', 'Temps perdu en recherches : ' + S.fmtH(res.hParAn) + ' par an, soit ' +
        S.fmtEUR(res.coutParAn) + ' (estimation), cumulés mois par mois.');
    }

    ready = true;
    update(true);
  }

  S.registerSim('temps', init);
})();

/* ═══ Simulateur 4 : Exposition au risque (les cinq détecteurs du produit) ═══ */
(function () {
  'use strict';
  var S = window.IbSim;
  if (!S) return;

  // Score : estimation paramétrable. Règles des détecteurs : celles du moteur (backend/internal/engine/detectors.go),
  // valeurs par défaut en l'absence de calibration apprise.
  var P_RETARD = 0.6, P_SILENCE = 0.4;   // poids des deux taux dans le score
  var NORM = 30;                          // pondéré (en points de %) donnant 100 à volume maximal
  var VOL_MIN = 0.4, VOL_MAX = 200;       // effet du volume : 0,4 à 1 (échelle logarithmique)
  var SEUIL_MOYEN = 33, SEUIL_ELEVE = 66; // libellés : < 33 faible, < 66 moyen, sinon élevé
  var JOURS_GRILLE = 14;

  var DETECTEURS = [
    { id: 'echeance', nom: 'Échéance à risque', regle: 'échéance confirmée dans les 7 jours, sans signal de progression récent ; critique à 2 jours',
      jours: 7, critique: 2, part: function (v) { return v.tauxRetard / 100; } },
    { id: 'silence', nom: 'Silence anormal', regle: 'fil sans réponse au-delà de deux fois son rythme habituel ; 4 jours par défaut, jamais moins de 24 h',
      jours: 4, part: function (v) { return v.tauxSilence / 100; } },
    { id: 'contradiction', nom: 'Promesse menacée', regle: 'promesse aval due dans les 7 jours, dont la promesse amont est en retard ou silencieuse',
      jours: 7, part: function (v) { return Math.min(v.tauxRetard, v.tauxSilence) / 100; } },
    { id: 'orphelin', nom: 'Engagement oublié', regle: 'engagement ouvert sans aucune suite 5 jours après sa création',
      jours: 5, part: null },
    { id: 'surcharge', nom: 'Surcharge', regle: '5 échéances ou plus la même semaine pour la même personne',
      jours: 5, semaine: true, part: null }
  ];

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
    return { score: score, lib: lib, aRisque: aRisque };
  }

  function init(root) {
    var corps = root.querySelector('.sim-corps');
    if (!corps) return;
    var vals = {}, ready = false;

    corps.appendChild(el('p', 'sim-etiquette', 'Règles réelles du produit, volumes estimés d\'après vos réglages'));

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

    // Grille des règles : une ligne par détecteur, une case par jour ; les cases pleines sont la fenêtre
    // dans laquelle le détecteur se déclenche (en jours avant l'échéance, ou en jours de silence).
    var CELL = 11, GAP = 3, PADL = 124, PADT = 16, ROWH = 22, LEG = 18;
    var W = PADL + JOURS_GRILLE * (CELL + GAP) - GAP + 2, H = PADT + DETECTEURS.length * ROWH + LEG;
    var g = S.svg('g', { 'aria-hidden': 'true' });
    DETECTEURS.forEach(function (d, i) {
      var y = PADT + i * ROWH;
      g.appendChild(S.svg('text', { x: 0, y: y + CELL - 1, 'class': 'sim-lib' }, d.nom));
      var n = d.semaine ? 7 : JOURS_GRILLE;
      for (var j = 0; j < n; j++) {
        var plein = d.semaine ? j < d.jours : (d.critique ? j < d.jours : j >= d.jours);
        var critique = d.critique && j < d.critique;
        g.appendChild(S.svg('rect', {
          x: PADL + j * (CELL + GAP), y: y, width: CELL, height: CELL, rx: 2,
          'class': 'sim-regle-case' + (plein ? (critique ? ' sim-regle-critique' : ' sim-regle-pleine') : '')
        }));
      }
      if (d.semaine) g.appendChild(S.svg('text', { x: PADL + 7 * (CELL + GAP) + 2, y: y + CELL - 1, 'class': 'sim-lib' }, '5 sur 7 jours'));
    });
    var legY = H - 4;
    var legende = S.svg('g', { 'aria-hidden': 'true' }, [
      S.svg('text', { x: PADL, y: 10, 'class': 'sim-lib' }, 'jour 1'),
      S.svg('text', { x: W - 2, y: 10, 'text-anchor': 'end', 'class': 'sim-lib' }, 'jour ' + JOURS_GRILLE),
      S.svg('text', { x: 0, y: legY, 'class': 'sim-lib' }, 'case pleine : le détecteur se déclenche')
    ]);
    var chartSvg = S.svg('svg', { viewBox: '0 0 ' + W + ' ' + H, role: 'img',
      'aria-label': 'Fenêtres de déclenchement des cinq détecteurs : échéance à risque dans les 7 jours avant l\'échéance (critique à 2 jours), silence anormal à partir de 4 jours, promesse menacée dans les 7 jours, engagement oublié à partir de 5 jours, surcharge à partir de 5 échéances la même semaine.' }, [g, legende]);
    var chart = el('div', 'sim-chart');
    chart.appendChild(chartSvg);
    corps.appendChild(chart);

    var topBox = el('div', 'sim-top');
    topBox.appendChild(el('p', 'sim-top-titre', 'Les cinq détecteurs, et ce qu\'ils toucheraient dans votre volume'));
    var ul = el('ul', 'sim-top-liste');
    var lis = [];
    DETECTEURS.forEach(function (d) {
      var li = el('li', 'sim-regle');
      var tete = el('span', 'sim-regle-tete');
      var nom = el('b', null, d.nom), nb = el('span', 'sim-top-n');
      tete.appendChild(nom); tete.appendChild(nb);
      li.appendChild(tete);
      li.appendChild(el('span', 'sim-regle-texte', d.regle));
      ul.appendChild(li);
      lis.push({ d: d, nb: nb });
    });
    topBox.appendChild(ul);
    corps.appendChild(topBox);
    corps.appendChild(el('p', 'sim-aide',
      'Les règles et leurs seuils sont ceux du moteur de détection, avant calibration sur vos échanges (le silence, par exemple, s\'ajuste au rythme réel de chaque fil). ' +
      'Les effectifs par détecteur sont estimés à partir de vos réglages, pas mesurés.'));

    var det = document.createElement('details');
    det.className = 'sim-formule';
    det.appendChild(el('summary', null, 'Voir le calcul'));
    det.appendChild(el('p', 'sim-formule-texte',
      'Score = 100 × (' + P_RETARD + ' × part en retard + ' + P_SILENCE + ' × part sans réponse) ÷ ' + NORM +
      ' × effet du volume, borné entre 0 et 100. L\'effet du volume croît de ' + VOL_MIN + ' à 1 selon le logarithme du nombre d\'engagements (1 à ' + VOL_MAX + '). ' +
      'Niveau : faible sous ' + SEUIL_MOYEN + ', moyen sous ' + SEUIL_ELEVE + ', élevé au-delà. ' +
      'Engagements à risque = engagements × (part en retard + part sans réponse) ÷ 100, plafonné au total. ' +
      'Par détecteur : échéance à risque ≈ engagements × part en retard ; silence anormal ≈ engagements × part sans réponse ; ' +
      'promesse menacée ≈ engagements × la plus petite des deux parts. Engagement oublié et surcharge dépendent de vos fils, pas de ces réglages.'));
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
      lis.forEach(function (l) {
        if (!l.d.part) { l.nb.textContent = 'selon vos fils'; return; }
        var n = vals.engagements * l.d.part(vals);
        l.nb.textContent = Math.round(n) === 0 ? 'aucun avec ces réglages' : '≈ ' + S.fmtInt(n) + ' engagement' + (Math.round(n) > 1 ? 's' : '');
      });
    }

    ready = true;
    update(true);
  }

  S.registerSim('risque', init);
})();

/* ═══ Simulateur 5 : Validation (scénario de démonstration, glisser + boutons) ═══ */
(function () {
  'use strict';
  var S = window.IbSim;
  if (!S) return;

  // Hypothèse : estimation du temps gagné par message validé en un clic, jamais une mesure.
  var TEMPS_PAR_VALIDATION = 3; // minutes
  var SEUIL_GLISSE = 100;       // px de glissement horizontal pour décider
  var DUREE_SORTIE = 260;       // ms, animation de sortie (ignorée si reduced-motion)

  // Les cinq brouillons que l'agent a réellement rédigés (6 novembre 2026, scénario de démonstration
  // « Menuiserie Dupont », fixtures/messages.json : seize messages fictifs d'une menuiserie). Rien n'est envoyé ici.
  var MESSAGES = [
    { de: 'À : commandes@vitrages-pro.fr', objet: 'Relance : livraison des 12 vitrages doubles – échéance dépassée',
      corps: '« Bonjour, La livraison des 12 vitrages doubles pour notre atelier était prévue au plus tard le 03/11. À ce jour, nous n’avons toujours pas reçu les éléments ni d’information sur le retard. Pouvez-vous nous indiquer où en est la com… »',
      prop: 'Motif : Livraison de 12 vitrages doubles à l\'atelier' },
    { de: 'À : paul.rossi@gmail.com', objet: 'Devis pergola bioclimatique – Envoi comme convenu',
      corps: '« Bonjour M. Rossi, Comme convenu lors de notre échange du 25 octobre, je vous transmets par ce mail le devis détaillé pour la pergola bioclimatique. Vous y trouverez l’ensemble des spécifications techniques, les délais et les condi… »',
      prop: 'Motif : Envoi du devis pour la pergola bioclimatique' },
    { de: 'À : famille.menard@orange.fr', objet: 'Rappel : réglage de votre porte d\'entrée sous garantie',
      corps: '« Bonjour, Je me permets de revenir vers vous concernant le réglage de votre porte d\'entrée, prévu sous garantie. Comme convenu le 30 octobre, je devais vous appeler avant de passer cette semaine. Pourriez-vous me confirmer vos disp… »',
      prop: 'Motif : Réglage de la porte d\'entrée sous garantie' },
    { de: 'À : sophie.laurent@hotmail.fr', objet: 'Confirmation de rendez-vous pour les mesures des fenêtres - 12/11',
      corps: '« Bonjour, Je me permets de revenir vers vous concernant notre rendez-vous prévu le 12 novembre à 10h pour la prise de mesures de vos 8 fenêtres. Pourriez-vous me confirmer que ce créneau vous convient toujours ? Dans l’attente de v… »',
      prop: 'Motif : Rendez-vous pour prise de mesures fenêtres' },
    { de: 'À : services-techniques@mairie-valbonne.fr', objet: 'Relance - Livraison des vitrages pour l\'école de Valbonne',
      corps: '« Bonjour, Je me permets de revenir vers vous concernant la pose des 12 fenêtres à l\'école de Valbonne, prévue le 10/11. La livraison des vitrages doubles, attendue le 03/11 auprès de notre atelier, n\'a toujours pas eu lieu. Ce reta… »',
      prop: 'Motif : Pose de 12 fenêtres à l\'école' }
  ];

  function el(tag, cls, text) {
    var e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text != null) e.textContent = text;
    return e;
  }

  function init(root) {
    var corps = root.querySelector('.sim-corps');
    if (!corps) return;
    var index = 0, valides = 0, passes = 0, busy = false;

    corps.appendChild(el('p', 'sim-etiquette', 'Brouillons rédigés par le produit sur le scénario de démonstration ; aucun envoi réel'));
    corps.appendChild(el('p', 'sim-aide', 'Les cinq relances que l\'agent a pré-rédigées sur le scénario « Menuiserie Dupont », telles qu\'elles apparaissent dans « À valider ». Glissez vers la droite pour envoyer, vers la gauche pour écarter, ou utilisez les boutons.'));

    var pile = el('div', 'sim-pile');
    corps.appendChild(pile);

    var actions = el('div', 'sim-actions');
    var btnPasser = el('button', 'sim-btn sim-btn-passer', 'Écarter');
    btnPasser.type = 'button';
    var btnValider = el('button', 'sim-btn sim-btn-valider', 'Envoyer');
    btnValider.type = 'button';
    var btnRecommencer = el('button', 'sim-btn sim-btn-valider', 'Recommencer');
    btnRecommencer.type = 'button';
    btnRecommencer.hidden = true;
    actions.appendChild(btnPasser); actions.appendChild(btnValider); actions.appendChild(btnRecommencer);
    corps.appendChild(actions);

    var statut = el('p', 'sim-sr');
    statut.setAttribute('role', 'status');
    corps.appendChild(statut);

    var sorties = el('div', 'sim-sorties');
    var boxV = el('div', 'sim-out'), outV = el('strong', null, '0');
    boxV.appendChild(outV); boxV.appendChild(el('span', null, 'Messages envoyés (simulation)'));
    var boxT = el('div', 'sim-out sim-out-ok'), outT = el('strong', null, '0 min');
    boxT.appendChild(outT); boxT.appendChild(el('span', null, 'Temps gagné (estimation)'));
    sorties.appendChild(boxV); sorties.appendChild(boxT);
    corps.appendChild(sorties);

    var det = document.createElement('details');
    det.className = 'sim-formule';
    det.appendChild(el('summary', null, 'Voir le calcul'));
    det.appendChild(el('p', 'sim-formule-texte',
      'Temps gagné = messages envoyés × ' + TEMPS_PAR_VALIDATION + ' minutes. Les ' + MESSAGES.length +
      ' brouillons ont été rédigés par Ibalya le 6 novembre 2026 sur le scénario de démonstration fourni avec le produit (une menuiserie fictive) ; rien n\'est envoyé ici, et la durée par validation est une hypothèse, pas une mesure.'));
    corps.appendChild(det);

    function majCompteurs() {
      outV.textContent = S.fmtInt(valides);
      outT.textContent = S.fmtInt(valides * TEMPS_PAR_VALIDATION) + ' min';
    }

    function carte(m, n) {
      var c = el('div', 'sim-carte');
      c.appendChild(el('span', 'sim-carte-de', m.de));
      c.appendChild(el('strong', 'sim-carte-objet', m.objet));
      c.appendChild(el('span', 'sim-carte-corps', m.corps));
      c.appendChild(el('span', 'sim-carte-prop', m.prop));
      c.appendChild(el('span', 'sim-carte-n', 'Message ' + n + ' sur ' + MESSAGES.length));
      return c;
    }

    var courante = null;

    function render() {
      pile.textContent = '';
      courante = null;
      var restants = MESSAGES.length - index;
      if (restants <= 0) {
        var fini = el('div', 'sim-pile-fini');
        fini.appendChild(el('strong', null, 'Terminé'));
        fini.appendChild(el('span', null, valides + ' envoyé' + (valides > 1 ? 's' : '') + ', ' + passes + ' écarté' + (passes > 1 ? 's' : '') + '.'));
        pile.appendChild(fini);
        btnValider.hidden = true; btnPasser.hidden = true; btnRecommencer.hidden = false;
        return;
      }
      btnValider.hidden = false; btnPasser.hidden = false; btnRecommencer.hidden = true;
      for (var k = Math.min(restants, 3) - 1; k >= 1; k--) {
        var g = el('div', 'sim-carte sim-carte-fond');
        g.setAttribute('aria-hidden', 'true');
        g.style.transform = 'translateY(' + (k * 8) + 'px) scale(' + (1 - k * 0.04) + ')';
        pile.appendChild(g);
      }
      courante = carte(MESSAGES[index], index + 1);
      courante.classList.add('sim-carte-haut');
      pile.appendChild(courante);
      brancherGlisse(courante);
    }

    function decider(valide) {
      if (busy || !courante) return;
      busy = true;
      if (valide) valides++; else passes++;
      majCompteurs();
      statut.textContent = (valide ? 'Envoyé : ' : 'Écarté : ') + MESSAGES[index].objet + '.';
      var c = courante;
      var avance = function () {
        index++;
        busy = false;
        var termine = index >= MESSAGES.length;
        render();
        if (termine) { statut.textContent += ' Tous les messages sont traités.'; btnRecommencer.focus(); }
      };
      if (S.prefersReducedMotion) { avance(); return; }
      c.classList.add('sim-carte-sortie');
      c.style.transform = 'translateX(' + (valide ? 140 : -140) + '%) rotate(' + (valide ? 12 : -12) + 'deg)';
      c.style.opacity = '0';
      setTimeout(avance, DUREE_SORTIE);
    }

    function brancherGlisse(c) {
      var depart = null, dx = 0;
      c.addEventListener('pointerdown', function (e) {
        if (busy || (e.pointerType === 'mouse' && e.button !== 0)) return;
        depart = e.clientX; dx = 0;
        c.classList.add('sim-carte-glisse');
        try { c.setPointerCapture(e.pointerId); } catch (err) { /* ignoré */ }
      });
      c.addEventListener('pointermove', function (e) {
        if (depart === null) return;
        dx = e.clientX - depart;
        c.style.transform = 'translateX(' + dx + 'px) rotate(' + (dx / 20).toFixed(1) + 'deg)';
        c.classList.toggle('sim-carte-oui', dx >= SEUIL_GLISSE);
        c.classList.toggle('sim-carte-non', dx <= -SEUIL_GLISSE);
      });
      c.addEventListener('pointerup', function () {
        if (depart === null) return;
        var d = dx; depart = null; dx = 0;
        c.classList.remove('sim-carte-glisse', 'sim-carte-oui', 'sim-carte-non');
        if (Math.abs(d) >= SEUIL_GLISSE) decider(d > 0);
        else c.style.transform = '';
      });
      c.addEventListener('pointercancel', function () {
        if (depart === null) return;
        depart = null; dx = 0;
        c.classList.remove('sim-carte-glisse', 'sim-carte-oui', 'sim-carte-non');
        c.style.transform = '';
      });
    }

    btnValider.addEventListener('click', function () { decider(true); });
    btnPasser.addEventListener('click', function () { decider(false); });
    btnRecommencer.addEventListener('click', function () {
      index = 0; valides = 0; passes = 0; busy = false;
      majCompteurs();
      statut.textContent = 'Réinitialisé : ' + MESSAGES.length + ' messages à traiter.';
      render();
      btnValider.focus();
    });

    majCompteurs();
    render();
  }

  S.registerSim('validation', init);
})();

/* ═══ Simulateur 6 : Fiabilité (la formule du produit, et ses mesures) ═══ */
(function () {
  'use strict';
  var S = window.IbSim;
  if (!S) return;

  // Formule réelle du score (backend/internal/engine/fiabilite.go, docs/FIABILITE.md) : une base, des signaux
  // vérifiables sans le modèle, l'avis du modèle plafonné, une pénalité d'historique.
  var BASE = 0.35;
  var SIGNAUX = [
    { id: 'echeance', label: 'Échéance explicite : une date écrite, pas déduite', poids: 0.20 },
    { id: 'formulation', label: 'Formulation engageante : « je vous livre jeudi », pas « il faudrait avancer »', poids: 0.15 },
    { id: 'fil', label: 'Fil conversationnel : au moins un aller-retour, pas une diffusion', poids: 0.10 },
    { id: 'interlocuteur', label: 'Interlocuteur connu : au moins trois échanges antérieurs', poids: 0.10 }
  ];
  var AVIS_MAX = 0.10;        // l'avis du modèle ne peut plus emporter la décision
  var PENALITE_MAX = 0.20;    // part des engagements de cet interlocuteur déjà corrigés
  var SEUIL_PUBLICATION = 0.50;
  var NIVEAUX = [{ min: 0.75, nom: 'Élevée', cls: 'sim-out-ok' }, { min: 0.50, nom: 'À vérifier', cls: 'sim-out-warn' }, { min: 0, nom: 'Incertaine', cls: 'sim-out-risk' }];

  // Mesures, sans embellir. Corpus d'évaluation : 12 messages étiquetés (fixtures/corpus_eval.json),
  // commande « evaluer » lancée le 6 novembre 2026. Base de recette : 28 engagements recalculés (docs/FIABILITE.md).
  var MESURES = [
    { nom: 'Pré-filtre', val: '2 écartés à raison, 0 à tort' },
    { nom: 'Précision', val: '100 % (5 engagements sur 5)' },
    { nom: 'Rappel', val: '83 % (5 sur 6 : un mail transféré manqué)' },
    { nom: 'Échéances exactes', val: '75 % (3 sur 4)' }
  ];
  var RECETTE = { total: 28, avant: { elevee: 23, autre: 5 }, apres: { elevee: 15, autre: 13 } };

  var CURSEURS = [
    { id: 'avis', label: 'Avis du modèle sur sa propre certitude', min: 0, max: 100, val: 80, step: 5, unit: '%' },
    { id: 'corrigees', label: 'Engagements de cet interlocuteur déjà corrigés', min: 0, max: 100, val: 0, step: 5, unit: '%' }
  ];

  function el(tag, cls, text) {
    var e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text != null) e.textContent = text;
    return e;
  }
  function fin(x) { return isFinite(x) ? x : 0; }
  function fmtScore(x) { return x.toFixed(2).replace('.', ','); }

  function calc(actifs, v) {
    var s = BASE;
    SIGNAUX.forEach(function (sig) { if (actifs[sig.id]) s += sig.poids; });
    s += AVIS_MAX * fin(v.avis) / 100;
    s -= PENALITE_MAX * fin(v.corrigees) / 100;
    s = Math.min(1, Math.max(0, s));
    var niveau = NIVEAUX[2];
    for (var i = 0; i < NIVEAUX.length; i++) if (s >= NIVEAUX[i].min) { niveau = NIVEAUX[i]; break; }
    return { score: s, niveau: niveau, publie: s >= SEUIL_PUBLICATION };
  }

  function init(root) {
    var corps = root.querySelector('.sim-corps');
    if (!corps) return;
    var actifs = {}, vals = {}, ready = false;

    corps.appendChild(el('p', 'sim-etiquette', 'Formule réelle du produit ; mesures du 6 novembre 2026'));

    var fs = document.createElement('fieldset');
    fs.className = 'sim-signaux';
    fs.appendChild(el('legend', null, 'Signaux vérifiables, sans le modèle'));
    SIGNAUX.forEach(function (s) {
      actifs[s.id] = true;
      var ligne = el('div', 'sim-champ sim-toggle');
      var inp = document.createElement('input');
      inp.type = 'checkbox'; inp.id = 'sim6-' + s.id; inp.checked = true;
      var lab = el('label', null, s.label + ' (+' + fmtScore(s.poids) + ')');
      lab.setAttribute('for', inp.id);
      ligne.appendChild(inp); ligne.appendChild(lab);
      fs.appendChild(ligne);
      inp.addEventListener('change', function () { actifs[s.id] = inp.checked; if (ready) update(false); });
    });
    corps.appendChild(fs);

    CURSEURS.forEach(function (c) {
      var champ = el('div', 'sim-champ');
      var lab = el('label', null, c.label);
      lab.setAttribute('for', 'sim6-' + c.id);
      var ligne = el('div', 'sim-range-ligne');
      var inp = document.createElement('input');
      inp.type = 'range'; inp.className = 'sim-range'; inp.id = 'sim6-' + c.id;
      inp.min = c.min; inp.max = c.max; inp.step = c.step; inp.value = c.val;
      inp.setAttribute('data-unit', c.unit);
      ligne.appendChild(inp);
      champ.appendChild(lab); champ.appendChild(ligne);
      corps.appendChild(champ);
      vals[c.id] = c.val;
      S.bindRange(inp, function (v) { vals[c.id] = fin(v); if (ready) update(false); });
    });

    var sorties = el('div', 'sim-sorties');
    var boxS = el('div', 'sim-out'), sS = el('strong', null, '—');
    boxS.appendChild(sS); boxS.appendChild(el('span', null, 'Score (0 à 1)'));
    var boxN = el('div', 'sim-out'), sN = el('strong', null, '—');
    boxN.appendChild(sN); boxN.appendChild(el('span', null, 'Niveau affiché'));
    var boxP = el('div', 'sim-out'), sP = el('strong', null, '—');
    boxP.appendChild(sP); boxP.appendChild(el('span', null, 'Publié (seuil ' + fmtScore(SEUIL_PUBLICATION) + ')'));
    sorties.appendChild(boxS); sorties.appendChild(boxN); sorties.appendChild(boxP);
    corps.appendChild(sorties);

    // Échelle du score : trois bandes (incertaine, à vérifier, élevée) et le repère du score courant.
    var W = 320, H = 78, PADL = 4, PADR = 4, BAR_Y = 26, BAR_H = 14;
    var bw = W - PADL - PADR;
    function x(v) { return PADL + bw * v; }
    var bandes = S.svg('g', { 'aria-hidden': 'true' }, [
      S.svg('rect', { x: x(0), y: BAR_Y, width: (x(0.5) - x(0)).toFixed(1), height: BAR_H, rx: 3, 'class': 'sim-bande sim-bande-risk' }),
      S.svg('rect', { x: x(0.5), y: BAR_Y, width: (x(0.75) - x(0.5)).toFixed(1), height: BAR_H, 'class': 'sim-bande sim-bande-warn' }),
      S.svg('rect', { x: x(0.75), y: BAR_Y, width: (x(1) - x(0.75)).toFixed(1), height: BAR_H, rx: 3, 'class': 'sim-bande sim-bande-ok' }),
      S.svg('text', { x: x(0), y: BAR_Y + BAR_H + 14, 'class': 'sim-lib' }, '0'),
      S.svg('text', { x: x(0.5), y: BAR_Y + BAR_H + 14, 'text-anchor': 'middle', 'class': 'sim-lib' }, '0,50'),
      S.svg('text', { x: x(0.75), y: BAR_Y + BAR_H + 14, 'text-anchor': 'middle', 'class': 'sim-lib' }, '0,75'),
      S.svg('text', { x: x(1), y: BAR_Y + BAR_H + 14, 'text-anchor': 'end', 'class': 'sim-lib' }, '1'),
      S.svg('text', { x: x(0.25), y: BAR_Y + BAR_H + 28, 'text-anchor': 'middle', 'class': 'sim-lib' }, 'incertaine'),
      S.svg('text', { x: x(0.625), y: BAR_Y + BAR_H + 28, 'text-anchor': 'middle', 'class': 'sim-lib' }, 'à vérifier'),
      S.svg('text', { x: x(0.875), y: BAR_Y + BAR_H + 28, 'text-anchor': 'middle', 'class': 'sim-lib' }, 'élevée')
    ]);
    var marqueur = S.svg('g', { 'class': 'sim-marqueur' }, [
      S.svg('line', { x1: 0, y1: BAR_Y - 6, x2: 0, y2: BAR_Y + BAR_H + 4, 'class': 'sim-marqueur-trait' }),
      S.svg('text', { x: 0, y: BAR_Y - 10, 'text-anchor': 'middle', 'class': 'sim-val' }, '')
    ]);
    var marqTxt = marqueur.lastChild;
    var echelle = S.svg('svg', { viewBox: '0 0 ' + W + ' ' + H, role: 'img', 'aria-label': '' }, [bandes, marqueur]);
    var chart = el('div', 'sim-chart');
    chart.appendChild(echelle);
    corps.appendChild(chart);

    // Ce que la formule a changé sur la base de recette : deux barres empilées, avant et après.
    var W2 = 320, H2 = 112, L2 = 4, R2 = 4, BH = 12;
    function barres(y, d, etiquette) {
      var w = W2 - L2 - R2, we = w * d.elevee / RECETTE.total, wa = w - we;
      return S.svg('g', {}, [
        S.svg('text', { x: L2, y: y + 10, 'class': 'sim-lib' }, etiquette + ' : ' + d.elevee + ' élevée · ' + d.autre + ' à vérifier ou incertaine'),
        S.svg('rect', { x: L2, y: y + 16, width: we.toFixed(1), height: BH, rx: 3, 'class': 'sim-bande-ok sim-bande' }),
        S.svg('rect', { x: (L2 + we).toFixed(1), y: y + 16, width: wa.toFixed(1), height: BH, rx: 3, 'class': 'sim-bande-warn sim-bande' })
      ]);
    }
    var recette = S.svg('svg', { viewBox: '0 0 ' + W2 + ' ' + H2, role: 'img',
      'aria-label': 'Base de recette, ' + RECETTE.total + ' engagements : avant la formule ' + RECETTE.avant.elevee + ' en fiabilité élevée et ' + RECETTE.avant.autre + ' à vérifier ou incertaine ; après, ' + RECETTE.apres.elevee + ' et ' + RECETTE.apres.autre + '.' }, [
      S.svg('text', { x: 0, y: 12, 'class': 'sim-lib' }, 'Base de recette, ' + RECETTE.total + ' engagements recalculés'),
      barres(24, RECETTE.avant, 'avant'),
      barres(24 + 44, RECETTE.apres, 'après')
    ]);
    var chart2 = el('div', 'sim-chart');
    chart2.appendChild(recette);
    corps.appendChild(chart2);

    var mesures = el('div', 'sim-top');
    mesures.appendChild(el('p', 'sim-top-titre', 'Mesuré sur le corpus d\'évaluation (12 messages étiquetés, 6 novembre 2026)'));
    var ul = el('ul', 'sim-top-liste');
    MESURES.forEach(function (m) {
      var li = el('li');
      li.appendChild(el('span', 'sim-top-nom', m.nom));
      li.appendChild(el('span', 'sim-top-n', m.val));
      ul.appendChild(li);
    });
    mesures.appendChild(ul);
    corps.appendChild(mesures);
    corps.appendChild(el('p', 'sim-aide',
      'Le corpus est petit et entièrement fictif (douze messages écrits pour l\'épreuve, aucun message client). Les vingt-huit engagements de recette ne suffisent pas non plus à conclure : ' +
      'ces chiffres disent où en est l\'extraction, pas ce qu\'elle vaudra sur votre boîte.'));

    var det = document.createElement('details');
    det.className = 'sim-formule';
    det.appendChild(el('summary', null, 'Voir le calcul'));
    det.appendChild(el('p', 'sim-formule-texte',
      'Score = ' + fmtScore(BASE) + ' de base + ' + SIGNAUX.map(function (s) { return fmtScore(s.poids) + ' si ' + s.label.split(' :')[0].toLowerCase(); }).join(' + ') +
      ' + jusqu\'à ' + fmtScore(AVIS_MAX) + ' selon l\'avis du modèle, moins jusqu\'à ' + fmtScore(PENALITE_MAX) + ' selon la part des engagements de cet interlocuteur déjà corrigés ' +
      '(pénalité appliquée dans le produit à partir de quatre engagements avec la même personne). Borné entre 0 et 1. ' +
      'Niveaux : élevée à partir de 0,75, à vérifier à partir de 0,50, incertaine en dessous ; seuil de publication 0,50. ' +
      'Un engagement sans échéance, sans formulation engageante, venant d\'un inconnu, plafonne à 0,45 même si le modèle annonce 1,00.'));
    corps.appendChild(det);

    function update(animate) {
      var res = calc(actifs, vals);
      if (sS._ibRaf) { cancelAnimationFrame(sS._ibRaf); sS._ibRaf = 0; }
      if (animate) S.animateCount(sS, 0, res.score, fmtScore);
      else sS.textContent = fmtScore(res.score);
      sN.textContent = res.niveau.nom;
      sP.textContent = res.publie ? 'Oui' : 'Non';
      boxS.className = 'sim-out ' + res.niveau.cls;
      boxN.className = 'sim-out ' + res.niveau.cls;
      boxP.className = 'sim-out ' + (res.publie ? 'sim-out-ok' : 'sim-out-risk');
      var px = x(res.score);
      marqueur.setAttribute('transform', 'translate(' + px.toFixed(1) + ' 0)');
      marqTxt.setAttribute('x', (Math.min(W - 16, Math.max(16, px)) - px).toFixed(1));
      marqTxt.textContent = fmtScore(res.score);
      echelle.setAttribute('aria-label', 'Score de fiabilité ' + fmtScore(res.score) + ' sur 1, niveau ' + res.niveau.nom.toLowerCase() + ', ' + (res.publie ? 'publié' : 'non publié') + '.');
    }

    ready = true;
    update(true);
  }

  S.registerSim('fiabilite', init);
})();
