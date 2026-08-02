// Tableau de bord AgentOps — vanilla JS, aucune dépendance.
let TOKEN = localStorage.getItem('agentops_token') || '';

const $ = (id) => document.getElementById(id);
const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));

async function api(path, opts = {}) {
  const resp = await fetch('/api' + path, {
    ...opts,
    headers: { 'Authorization': 'Bearer ' + TOKEN, 'Content-Type': 'application/json', ...(opts.headers || {}) },
  });
  if (resp.status === 401) { showLogin('Jeton invalide.'); throw new Error('401'); }
  const data = await resp.json().catch(() => ({}));
  if (!resp.ok) throw new Error(data.error || resp.statusText);
  return data;
}

function toast(msg, isError = false) {
  const t = $('toast');
  t.textContent = msg;
  t.className = 'toast' + (isError ? ' error-toast' : '');
  setTimeout(() => t.classList.add('hidden'), 4000);
}

function showLogin(err = '') {
  $('login').classList.remove('hidden');
  $('app').classList.add('hidden');
  $('login-error').textContent = err;
}

function saveToken() {
  TOKEN = $('token-input').value.trim();
  localStorage.setItem('agentops_token', TOKEN);
  boot();
}

// --- navigation ---
document.addEventListener('click', (e) => {
  const tab = e.target.closest('nav button');
  if (!tab) return;
  document.querySelectorAll('nav button').forEach((b) => b.classList.remove('active'));
  tab.classList.add('active');
  document.querySelectorAll('main section').forEach((s) => s.classList.add('hidden'));
  $('tab-' + tab.dataset.tab).classList.remove('hidden');
  loaders[tab.dataset.tab]?.();
});

const fmtDate = (s) => s ? new Date(s).toLocaleDateString('fr-FR') : '—';
const fmtDT = (s) => s ? new Date(s).toLocaleString('fr-FR') : '—';

const STATUT_LABELS = { ouvert: 'Ouvert', confirme: 'Confirmé', livre: 'Livré', en_retard: 'En retard', abandonne: 'Abandonné' };
const DET_LABELS = { echeance_a_risque: 'Échéance à risque', silence_anormal: 'Silence anormal', contradiction: 'Contradiction', orphelin: 'Orphelin', surcharge: 'Surcharge' };
const TYPE_LABELS = { devis: '📋 Devis', livraison: '📦 Livraison', relance: '🔁 Relance', prise_de_contact: '🤝 Prise de contact', rendez_vous: '📆 Rendez-vous', facturation: '💶 Facturation', autre: 'Autre' };

// --- Aperçu ---
async function loadApercu() {
  try {
    const st = await api('/status');
    const c = st.compteurs;
    $('status-pill').textContent = (st.canal_connecte ? '● ' : '○ ') + st.canal + (st.compte ? ' · ' + st.compte : '') + (st.service_llm_ok ? ' · LLM ok' : ' · LLM injoignable');
    $('status-pill').className = 'pill ' + (st.canal_connecte && st.service_llm_ok ? 'ok' : 'warn');
    $('compteurs').innerHTML = [
      ['Messages lus', c.messages], ['En attente d\'analyse', c.messages_en_attente],
      ['Exclus (pré-filtre)', c.messages_exclus], ['Engagements', c.engagements],
      ['Détections actives', c.detections_actives], ['Brouillons proposés', c.brouillons_proposes],
    ].map(([l, v]) => `<div class="card"><div class="num">${v}</div><div class="lbl">${l}</div></div>`).join('');

    const dets = await api('/detections?proactives=1');
    const critiques = (dets || []).filter((d) => d.critique);
    $('alertes').innerHTML = critiques.length
      ? `<div class="alert-band">⚠ ${critiques.length} alerte(s) critique(s)</div>` +
        critiques.map((d) => `<div class="item critical"><b>${esc(d.titre)}</b><p>${esc(d.detail)}</p></div>`).join('')
      : '';
  } catch (e) { /* status pill reste */ }
  try {
    const rep = await api('/digest/latest?type=' + (localStorage.getItem('digest_type') || 'quotidien'));
    renderDigest(rep);
  } catch { $('digest').innerHTML = '<p class="muted">Aucun digest encore généré.</p>'; }
}

function renderDigest(rep) {
  const d = rep.content || rep;
  let html = `<p class="muted">Généré le ${fmtDT(d.genere_le)}</p>`;
  if ((d.detections || []).length) {
    html += '<h3>Alertes du jour</h3>' + d.detections.map((x) =>
      `<div class="item ${x.critique ? 'critical' : ''}"><span class="tag">${DET_LABELS[x.type] || x.type}</span> <b>${esc(x.titre)}</b><p>${esc(x.detail)}</p></div>`).join('');
  }
  if ((d.engagements_a_risque || []).length) {
    html += '<h3>Engagements à risque</h3>' + d.engagements_a_risque.map((e) =>
      `<div class="item"><b>${esc(e.objet)}</b><p>${esc(e.emetteur_email)} → ${esc(e.destinataire_email)} · échéance ${fmtDate(e.echeance)} · ${STATUT_LABELS[e.statut]}</p></div>`).join('');
  }
  if ((d.brouillons_proposes || []).length) {
    html += `<h3>Suggestions d'action</h3><p class="muted">${d.brouillons_proposes.length} brouillon(s) — voir l'onglet Brouillons pour valider d'un clic.</p>`;
  }
  if (!(d.detections || []).length && !(d.engagements_a_risque || []).length) {
    html += '<p class="muted">Rien à signaler au-dessus du seuil. 👌</p>';
  }
  $('digest').innerHTML = html;
}

async function runCycle() {
  toast('Cycle en cours…');
  try { const r = await api('/cycle/run', { method: 'POST', body: JSON.stringify({ since_days: 2 }) });
    toast(r.erreur ? 'Cycle terminé avec erreur : ' + r.erreur : 'Cycle terminé (' + r.duree + ')', !!r.erreur);
    loadApercu();
  } catch (e) { toast(e.message, true); }
}

async function genDigest() {
  try { await api('/digest/generate', { method: 'POST', body: JSON.stringify({ type: localStorage.getItem('digest_type') || 'quotidien' }) });
    toast('Digest généré'); loadApercu();
  } catch (e) { toast(e.message, true); }
}

// --- Pilotage (vue CODIR) ---
function miniEng(e, action) {
  const ech = e.echeance ? fmtDate(e.echeance) : 'sans échéance';
  return `<div class="item">
    <span class="tag type">${TYPE_LABELS[e.type] || e.type}</span>
    ${e.priorite === 'haute' ? '<span class="tag prio">Priorité haute</span>' : ''}
    <b>${esc(e.objet)}</b>
    <p>${esc(e.destinataire_email || e.emetteur_email || '')} · ${ech}</p>
    ${action || ''}
  </div>`;
}

async function loadPilotage() {
  try {
    const p = await api('/pilotage');
    $('pilotage-alertes').innerHTML = (p.alertes_critiques || []).map((d) =>
      `<div class="item critical"><span class="tag">${DET_LABELS[d.type] || d.type}</span> <b>${esc(d.titre)}</b><p>${esc(d.detail)}</p></div>`).join('');

    const types = Object.entries(p.par_type || {}).sort((a, b) => b[1] - a[1]);
    $('pilotage-types').innerHTML = types.map(([t, n]) =>
      `<span class="chip">${TYPE_LABELS[t] || t} <b>${n}</b></span>`).join('') || '<p class="muted">Aucun engagement actif.</p>';

    $('pilotage-retard').innerHTML = (p.en_retard || []).map((e) => miniEng(e,
      `<div class="row-actions"><button onclick="patchEng(${e.id}, {statut:'livre'})">✓ C'est fait</button>
       <button onclick="correct(${e.id}, 'pas_un_engagement')">✗ Pas un engagement</button></div>`)).join('')
      || '<p class="muted">Rien en retard. 👌</p>';

    $('pilotage-jalons').innerHTML = (p.jalons_14_jours || []).map((e) => miniEng(e)).join('')
      || '<p class="muted">Aucun jalon confirmé sur les 14 prochains jours.</p>';

    const qw = p.quick_wins || {};
    let html = '';
    for (const d of qw.brouillons_a_valider || []) {
      html += `<div class="item draft"><span class="tag">✉ brouillon prêt</span> <b>${esc(d.subject)}</b>
        <p>À ${esc(d.to_email)}${d.detection_titre ? ' · ' + esc(d.detection_titre) : ''}</p>
        <div class="row-actions"><button class="primary" onclick="validateDraft(${d.id})">✓ Valider et envoyer</button>
        <button onclick="switchTab('brouillons')">Voir / modifier</button></div></div>`;
    }
    for (const e of qw.echeances_a_confirmer || []) {
      html += miniEng(e, `<div class="row-actions"><button class="primary" onclick="confirmEcheance(${e.id}, '${(e.echeance || '').slice(0, 10)}')">📅 Confirmer l'échéance</button></div>`);
    }
    for (const l of qw.liens_a_trancher || []) {
      html += `<div class="item"><span class="tag candidat">lien à trancher</span>
        <p><b>${esc(l.amont_objet)}</b> conditionne-t-il <b>${esc(l.aval_objet)}</b> ?</p>
        <div class="row-actions"><button class="primary" onclick="linkAction(${l.id}, 'confirm').then(loadPilotage)">Oui</button>
        <button onclick="linkAction(${l.id}, 'reject').then(loadPilotage)">Non</button></div></div>`;
    }
    $('pilotage-quickwins').innerHTML = html || '<p class="muted">Aucune action en attente — tout est traité.</p>';
  } catch (e) { toast(e.message, true); }
}

function switchTab(name) {
  document.querySelector(`nav button[data-tab="${name}"]`)?.click();
}

// --- Miroir ---
async function loadMiroir() {
  try {
    const rep = await api('/miroir');
    const m = rep.content;
    let html = `<p class="muted">${esc(m.note)}</p>
      <p>Généré le ${fmtDT(m.genere_le)} — ${m.messages_lus} messages lus sur ${m.periode_jours} jours.</p>`;
    html += '<h3>Engagements en cours (' + (m.engagements_ouverts || []).length + ')</h3>' +
      ((m.engagements_ouverts || []).map(engagementCard).join('') || '<p class="muted">Aucun.</p>');
    html += '<h3>En retard probable (' + (m.en_retard_probable || []).length + ')</h3>' +
      ((m.en_retard_probable || []).map(engagementCard).join('') || '<p class="muted">Aucun.</p>');
    html += '<h3>Fils sans réponse</h3>' +
      ((m.fils_sans_reponse || []).map((f) => `<div class="item"><b>${esc(f.sujet)}</b><p>${esc(f.interlocuteur)} · silence depuis ${f.jours_silence} jours</p></div>`).join('') || '<p class="muted">Aucun.</p>');
    $('miroir').innerHTML = html;
  } catch { $('miroir').innerHTML = '<p class="muted">Aucun miroir généré. Connectez un canal puis lancez l\'onboarding (Réglages).</p>'; }
}

async function genMiroir() {
  try { await api('/miroir/generate', { method: 'POST' }); toast('Miroir régénéré'); loadMiroir(); }
  catch (e) { toast(e.message, true); }
}

// --- Engagements ---
function engagementCard(e) {
  const ech = e.echeance ? fmtDate(e.echeance) + (e.echeance_inferee && !e.echeance_confirmee ? ' <span class="tag warn-tag">inférée — à confirmer</span>' : '') : '—';
  return `<div class="item">
    <span class="tag type">${TYPE_LABELS[e.type] || e.type}</span>
    <span class="tag ${e.statut}">${STATUT_LABELS[e.statut] || e.statut}</span>
    ${e.priorite === 'haute' ? '<span class="tag prio">Priorité haute</span>' : ''}
    <b>${esc(e.objet)}</b>
    <p>${esc(e.emetteur_email || '?')} → ${esc(e.destinataire_email || '?')} · échéance ${ech} · confiance ${(e.confiance * 100).toFixed(0)} %</p>
    <div class="row-actions">
      <button onclick="patchEng(${e.id}, {statut:'livre'})">✓ Livré</button>
      <button onclick="correct(${e.id}, 'pas_un_engagement')">✗ Pas un engagement</button>
      <button onclick="correct(${e.id}, 'priorite_haute')">↑ Priorité haute</button>
      <button onclick="correct(${e.id}, 'ne_plus_alerter')">🔕 Ne plus alerter</button>
      ${e.echeance && !e.echeance_confirmee ? `<button onclick="confirmEcheance(${e.id}, '${(e.echeance || '').slice(0, 10)}')">📅 Confirmer l'échéance</button>` : ''}
    </div>
  </div>`;
}

async function loadEngagements() {
  const statut = $('filtre-statut').value;
  try {
    const engs = await api('/engagements' + (statut ? '?statut=' + statut : ''));
    $('engagements').innerHTML = (engs || []).map(engagementCard).join('') || '<p class="muted">Aucun engagement.</p>';
  } catch (e) { toast(e.message, true); }
}

async function patchEng(id, body) {
  try { await api('/engagements/' + id, { method: 'PATCH', body: JSON.stringify(body) }); toast('Corrigé — règle mémorisée'); loadEngagements(); }
  catch (e) { toast(e.message, true); }
}

async function correct(id, action) {
  try { const r = await api('/engagements/' + id + '/correct', { method: 'POST', body: JSON.stringify({ action }) });
    toast(r.regle ? 'Règle apprise : ' + r.regle : 'Corrigé'); loadEngagements();
  } catch (e) { toast(e.message, true); }
}

async function confirmEcheance(id, current) {
  const d = prompt('Confirmer ou corriger l\'échéance (AAAA-MM-JJ) :', current);
  if (!d) return;
  patchEng(id, { echeance: d });
}

// --- Détections ---
async function loadDetections() {
  try {
    const dets = await api('/detections');
    $('detections').innerHTML = (dets || []).map((d) => `
      <div class="item ${d.critique ? 'critical' : ''}">
        <span class="tag">${DET_LABELS[d.type] || d.type}</span>
        <span class="muted">score ${(d.score * 100).toFixed(0)} % · ${fmtDT(d.created_at)}</span>
        <b>${esc(d.titre)}</b><p>${esc(d.detail)}</p>
        <div class="row-actions"><button onclick="dismissDet(${d.id})">Écarter</button></div>
      </div>`).join('') || '<p class="muted">Aucune détection active.</p>';
  } catch (e) { toast(e.message, true); }
}

async function dismissDet(id) {
  try { await api('/detections/' + id + '/dismiss', { method: 'POST' }); toast('Détection écartée'); loadDetections(); }
  catch (e) { toast(e.message, true); }
}

// --- Brouillons ---
async function loadBrouillons() {
  try {
    const drafts = await api('/drafts?statut=propose');
    $('brouillons').innerHTML = (drafts || []).map((d) => `
      <div class="item draft" id="draft-${d.id}">
        ${d.detection_titre ? `<p class="context">💡 Pourquoi : ${esc(d.detection_titre)}${d.detection_detail ? ' — ' + esc(d.detection_detail) : ''}</p>` : ''}
        ${d.engagement_objet ? `<p class="context">🔗 Engagement concerné : ${esc(d.engagement_objet)}</p>` : ''}
        <div class="draft-view">
          <b>À : ${esc(d.to_email)}</b> — <b>${esc(d.subject)}</b>
          <pre>${esc(d.body)}</pre>
          <div class="row-actions">
            <button class="primary" onclick="validateDraft(${d.id})">✓ Valider et envoyer</button>
            <button onclick="editDraft(${d.id})">✎ Modifier</button>
            <button onclick="rejectDraft(${d.id})">✗ Rejeter</button>
          </div>
        </div>
        <div class="draft-edit hidden">
          <label>Destinataire</label><input id="draft-to-${d.id}" value="${esc(d.to_email)}">
          <label>Objet</label><input id="draft-subject-${d.id}" value="${esc(d.subject)}">
          <label>Message</label><textarea id="draft-body-${d.id}" rows="8">${esc(d.body)}</textarea>
          <div class="row-actions">
            <button class="primary" onclick="saveDraft(${d.id})">💾 Enregistrer</button>
            <button onclick="editDraft(${d.id})">Annuler</button>
          </div>
        </div>
      </div>`).join('') || '<p class="muted">Aucun brouillon en attente.</p>';
  } catch (e) { toast(e.message, true); }
}

function editDraft(id) {
  const card = $('draft-' + id);
  card.querySelector('.draft-view').classList.toggle('hidden');
  card.querySelector('.draft-edit').classList.toggle('hidden');
}

async function saveDraft(id) {
  try {
    await api('/drafts/' + id, { method: 'PATCH', body: JSON.stringify({
      to_email: $('draft-to-' + id).value.trim(),
      subject: $('draft-subject-' + id).value.trim(),
      body: $('draft-body-' + id).value,
    }) });
    toast('Brouillon enregistré');
    loadBrouillons();
  } catch (e) { toast(e.message, true); }
}

const draftsInFlight = new Set();
async function validateDraft(id) {
  if (draftsInFlight.has(id)) return;
  if (!confirm('Envoyer ce message maintenant ?')) return;
  draftsInFlight.add(id);
  try { await api('/drafts/' + id + '/validate', { method: 'POST' }); toast('Message envoyé ✓'); loadBrouillons(); }
  catch (e) { toast(e.message, true); }
  finally { draftsInFlight.delete(id); }
}

async function rejectDraft(id) {
  try { await api('/drafts/' + id + '/reject', { method: 'POST' }); toast('Brouillon rejeté'); loadBrouillons(); }
  catch (e) { toast(e.message, true); }
}

// --- Liens ---
async function loadLiens() {
  try {
    const links = await api('/links');
    $('liens').innerHTML = (links || []).map((l) => `
      <div class="item">
        <span class="tag ${l.statut}">${l.statut}</span>
        <p><b>${esc(l.amont_objet)}</b> ⟶ conditionne ⟶ <b>${esc(l.aval_objet)}</b></p>
        <p class="muted">${esc(l.raison)}</p>
        ${l.statut === 'candidat' ? `<div class="row-actions">
          <button class="primary" onclick="linkAction(${l.id}, 'confirm')">✓ Confirmer</button>
          <button onclick="linkAction(${l.id}, 'reject')">✗ Rejeter</button></div>` : ''}
      </div>`).join('') || '<p class="muted">Aucun lien de dépendance proposé.</p>';
  } catch (e) { toast(e.message, true); }
}

async function linkAction(id, action) {
  try { await api('/links/' + id + '/' + action, { method: 'POST' }); toast('Lien mis à jour'); loadLiens(); }
  catch (e) { toast(e.message, true); }
}

// --- Capsule ---
async function loadCapsule() {
  try {
    const c = await api('/capsule');
    $('capsule-facts').value = JSON.stringify(c.facts || {}, null, 2);
    const i = c.intentions || {};
    $('int-priorites').value = i.priorites || '';
    $('int-surveillance').value = i.surveillance || '';
    $('int-energie').value = i.energie || '';
  } catch (e) { toast(e.message, true); }
}

async function saveCapsule() {
  let facts;
  try { facts = JSON.parse($('capsule-facts').value); } catch { toast('JSON des faits invalide', true); return; }
  const intentions = {
    priorites: $('int-priorites').value.trim(),
    surveillance: $('int-surveillance').value.trim(),
    energie: $('int-energie').value.trim(),
  };
  try { await api('/capsule', { method: 'PUT', body: JSON.stringify({ facts, intentions }) }); toast('Capsule enregistrée'); }
  catch (e) { toast(e.message, true); }
}

async function inferCapsule() {
  toast('Inférence en cours…');
  try { await api('/capsule/infer', { method: 'POST' }); toast('Faits ré-inférés'); loadCapsule(); }
  catch (e) { toast(e.message, true); }
}

// --- Règles ---
async function loadRegles() {
  try {
    const rules = await api('/rules');
    $('regles').innerHTML = (rules || []).map((r) => `
      <div class="item ${r.active ? '' : 'inactive'}">
        <span class="tag">${esc(r.portee_type)}</span> <b>${esc(r.action)}</b> ${r.portee_cible ? '· ' + esc(r.portee_cible) : ''}
        <p>${esc(r.note)} <span class="muted">· ${fmtDate(r.created_at)}</span></p>
        ${r.active ? `<div class="row-actions"><button onclick="delRule(${r.id})">Désactiver</button></div>` : '<p class="muted">désactivée</p>'}
      </div>`).join('') || '<p class="muted">Aucune règle apprise pour l\'instant. Corrigez un engagement pour en créer une.</p>';
  } catch (e) { toast(e.message, true); }
}

async function delRule(id) {
  try { await api('/rules/' + id, { method: 'DELETE' }); toast('Règle désactivée'); loadRegles(); }
  catch (e) { toast(e.message, true); }
}

// --- KPI (CDC section 14) ---
async function loadKpis() {
  try {
    const k = await api('/kpis');
    const pct = (v) => (v * 100).toFixed(0) + ' %';
    const rows = [
      ['Précision estimée des extractions', pct(k.precision_estimee), 'cible > 85 %', k.precision_estimee > 0.85],
      ['Taux de faux positifs (digests)', pct(k.taux_faux_positifs), 'cible < 10 %', k.taux_faux_positifs < 0.10],
      ['Actions validées / suggérées', pct(k.taux_validation_actions), 'cible > 40 %', k.taux_validation_actions > 0.40],
      ['Corrections sur 7 jours', k.corrections_7_jours, 'cible < 3 après S3', k.corrections_7_jours < 3],
      ['Taux d\'exclusion pré-filtre', pct(k.taux_exclusion_prefiltre), 'santé économique EF-11', true],
      ['Engagements extraits', k.engagements_extraits, '', true],
      ['Messages analysés', k.messages_analyses, '', true],
      ['Digests générés', k.digests_generes, '', true],
      ['Règles apprises actives', k.regles_apprises_actives, '', true],
      ['Incidents critiques (action agent)', k.incidents_critiques, 'cible : 0', k.incidents_critiques === 0],
    ];
    $('kpis').innerHTML = rows.map(([l, v, cible, ok]) =>
      `<div class="card" style="border-left:3px solid ${ok ? 'var(--ok)' : 'var(--warn)'}">
        <div class="num">${v}</div><div class="lbl">${l}${cible ? ' · ' + cible : ''}</div></div>`).join('');
  } catch (e) { toast(e.message, true); }
}

// --- Audit ---
async function loadAudit() {
  try {
    const entries = await api('/audit?limit=200');
    $('audit').innerHTML = '<table><tr><th>Date</th><th>Acteur</th><th>Événement</th><th>Détails</th></tr>' +
      (entries || []).map((e) => `<tr><td>${fmtDT(e.ts)}</td><td>${esc(e.actor)}</td><td>${esc(e.event_type)}</td><td class="mono">${esc(JSON.stringify(e.payload))}</td></tr>`).join('') + '</table>';
  } catch (e) { toast(e.message, true); }
}

// --- Réglages ---
async function loadReglages() {
  try {
    const s = await api('/settings');
    $('set-seuil').value = s.seuil_publication;
    $('set-digest').value = s.digest_type;
    $('set-digest-email').value = s.digest_email || '0';
    const st = await api('/status');
    $('oauth-status').textContent = st.canal_connecte ? 'Connecté' + (st.compte ? ' : ' + st.compte : '') : 'Non connecté';
  } catch (e) { toast(e.message, true); }
}

async function saveSettings() {
  try {
    await api('/settings', { method: 'PUT', body: JSON.stringify({ seuil_publication: $('set-seuil').value, digest_type: $('set-digest').value, digest_email: $('set-digest-email').value }) });
    localStorage.setItem('digest_type', $('set-digest').value);
    toast('Réglages enregistrés');
  } catch (e) { toast(e.message, true); }
}

async function connectGmail() {
  try { const r = await api('/oauth/google/start', { method: 'POST' }); window.location.href = r.url; }
  catch (e) { toast(e.message, true); }
}

async function runOnboarding() {
  try { await api('/onboarding/run', { method: 'POST' }); toast('Onboarding lancé — le miroir sera prêt dans quelques minutes.'); }
  catch (e) { toast(e.message, true); }
}

const loaders = {
  apercu: loadApercu, pilotage: loadPilotage, miroir: loadMiroir, engagements: loadEngagements,
  detections: loadDetections, brouillons: loadBrouillons, liens: loadLiens,
  capsule: loadCapsule, regles: loadRegles, kpis: loadKpis, audit: loadAudit, reglages: loadReglages,
};

async function boot() {
  if (!TOKEN) { showLogin(); return; }
  try {
    await api('/status');
    $('login').classList.add('hidden');
    $('app').classList.remove('hidden');
    loadApercu();
  } catch (e) { if (e.message !== '401') showLogin(e.message); }
}

boot();
