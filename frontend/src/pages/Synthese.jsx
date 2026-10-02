import { useCallback, useEffect, useState } from 'react'
import Icone from '../components/Icone'
import { useNavigate } from 'react-router-dom'
import { api, toast, estVueCombinee, definirVueCombinee } from '../api'
import { DraftPanel, useDraft } from '../components/DraftPanel'
import SourcePanel from '../components/SourcePanel'
import Onboarding from '../components/Onboarding'
import { SqueletteKpi, SqueletteLignes } from '../components/Squelette'
import { DET_LABELS, Reli } from '../components/ui'
import { libelleCycle, useEtatAgent } from '../etatAgent'

const CAT_META = {
  encours: { dot: 'blue', titre: 'Dans les temps' },
  retard: { dot: 'amber', titre: 'En retard' },
  risque: { dot: 'red', titre: 'Retard probable' },
}

export default function Synthese() {
  const [syn, setSyn] = useState(null)
  const navigate = useNavigate()
  const combinee = estVueCombinee()

  const load = useCallback(() => {
    api(combinee ? '/synthese/global' : '/synthese').then(setSyn).catch((e) => toast(e.message, true))
  }, [combinee])
  useEffect(load, [load])

  // Vue combinée : chaque élément vit dans sa boîte (tenant isolé). Pour agir
  // dessus, on bascule la session sur cette boîte (et on quitte la vue combinée).
  const ouvrirBoite = async (boiteId, dest) => {
    definirVueCombinee(false)
    try {
      await api(`/espaces/${boiteId}/activer`, { method: 'POST' })
      if (dest) navigate(dest)
      window.location.reload()
    } catch (e) { toast(e.message, true) }
  }

  const d = useDraft(load)
  const [sourceId, setSourceId] = useState(null)
  const [alerteSrc, setAlerteSrc] = useState(null)
  const { cycle, finiA, rafraichir } = useEtatAgent()
  const enCours = !!cycle?.en_cours

  // Les données se rechargent dès qu'un cycle se termine, y compris celui que
  // le scheduler a lancé sans que le dirigeant ait rien demandé.
  useEffect(() => { if (finiA) load() }, [finiA, load])

  const runCycle = async () => {
    if (enCours) return
    rafraichir() // le bouton se verrouille sans attendre le prochain sondage
    try {
      const r = await api('/cycle/run', { method: 'POST', body: JSON.stringify({ since_days: 2 }) })
      toast(r.erreur ? 'Terminé avec erreur : ' + r.erreur : `Analyse terminée en ${r.duree}`, !!r.erreur)
      load()
    } catch (e) {
      // Un cycle long dépasse le délai du relais : la requête échoue alors que
      // l'analyse se poursuit côté serveur. Le sondage prend le relais et
      // rechargera la page à la fin ; inutile d'alarmer sur un faux échec.
      const st = await rafraichir()
      if (!st?.cycle?.en_cours) toast(e.message, true)
    } finally { rafraichir() }
  }

  const marquerLivre = async (id) => {
    try {
      await api(`/engagements/${id}`, { method: 'PATCH', body: JSON.stringify({ statut: 'livre' }) })
      toast('Marqué comme résolu'); load()
    } catch (e) { toast(e.message, true) }
  }

  const k = syn?.kpi
  const cats = syn?.categories || {}

  return (
    <section>
      <div className="page-head">
        <div>
          <h1>Synthèse{combinee ? ' · toutes les boîtes' : ''}</h1>
          <p>{combinee
            ? "Tout ce qui demande une décision, agrégé sur toutes vos boîtes. Chaque élément porte sa boîte ; cliquez pour l'ouvrir dans celle-ci et agir."
            : "L'essentiel de votre activité en un coup d'œil : ce qui bloque, ce qui arrive, ce que vous pouvez traiter sans quitter cette page."}</p>
        </div>
        {!combinee && (
          <div className="page-actions">
            <button className="btn" onClick={runCycle} disabled={enCours}
              title={enCours ? libelleCycle(cycle) : 'Lire les nouveaux messages et mettre à jour le suivi'}>
              {enCours ? <><span className="rotor" aria-hidden="true" />Analyse en cours…</> : <><Icone nom="action-analyser" /> Analyser</>}
            </button>
          </div>
        )}
      </div>

      {!combinee && <Onboarding />}

      {!combinee && enCours && (
        <div className="bandeau-cycle" role="status" aria-live="polite">
          <span className="rotor" aria-hidden="true" />
          <div>
            <b>{cycle.phase || 'Analyse en cours'}</b>
            <span className="sub">
              {cycle.origine === 'dirigeant'
                ? "Analyse que vous avez lancée"
                : "Analyse automatique, l'agent relit votre boîte toutes les 15 minutes"}
              {' · '}{libelleCycle(cycle).split(' · ')[1]}
            </span>
          </div>
        </div>
      )}

      {!syn && <SqueletteKpi />}
      {syn && combinee && <div className="kpi-row">
        <div className="kpi static"><span className="lbl">Actifs</span><span className="num">{k?.engagements_suivis ?? '—'}</span></div>
        <div className="kpi static warn"><span className="lbl">En retard</span><span className="num">{k?.retards ?? '—'}</span></div>
        <div className="kpi static risk flag"><span className="lbl">Critiques</span><span className="num">{k?.risques ?? '—'}</span></div>
        <div className="kpi static accent flag"><span className="lbl">À valider</span><span className="num">{k?.messages_a_valider ?? '—'}</span></div>
        <div className="kpi static"><span className="lbl">Messages lus / 30j</span><span className="num">{k?.messages_lus ?? '—'}</span></div>
      </div>}
      {syn && !combinee && <div className="kpi-row">
        <button className="kpi" onClick={() => navigate('/suivi')}>
          <span className="lbl">Actifs</span><span className="num">{k?.engagements_suivis ?? '–'}</span>
        </button>
        <button className="kpi warn" onClick={() => navigate('/suivi?cat=retard')}>
          <span className="lbl">En retard</span><span className="num">{k?.retards ?? '–'}</span>
        </button>
        <button className="kpi risk flag" onClick={() => navigate('/suivi?cat=risque')}>
          <span className="lbl">Retard probable</span><span className="num">{k?.risques ?? '–'}</span>
        </button>
        <button className="kpi accent flag" onClick={() => navigate('/a-valider')}>
          <span className="lbl">À valider</span><span className="num">{k?.messages_a_valider ?? '–'}</span>
        </button>
        <div className="kpi static">
          <span className="lbl">Messages lus / 30j</span><span className="num">{k?.messages_lus ?? '–'}</span>
        </div>
      </div>}

      <div className="section-title"><h2>Ce qui demande une décision maintenant</h2></div>
      <div className="priority-list">
        {!syn && <SqueletteLignes nombre={3} />}
        {syn && !syn.priorites?.length && <div className="empty">Rien à arbitrer, aucun retard ni engagement bloqué.</div>}
        {(syn?.priorites || []).map((p) => (
          <div className={'priority-item ' + p.categorie} key={(p.boite_id || 0) + '-' + p.engagement_id}>
            <span className={'p-badge ' + p.categorie}>{p.categorie === 'risque' ? 'Retard probable' : 'En retard'}</span>
            <div className="p-body">
              <p className="p-title">
                {combinee
                  ? <button className="lien-source" title={'Ouvrir dans ' + p.boite}
                      onClick={() => ouvrirBoite(p.boite_id, '/suivi')}>{p.titre}</button>
                  : <button className="lien-source" title="Voir la conversation d'origine"
                      onClick={() => setSourceId(p.engagement_id)}>{p.titre}</button>}
              </p>
              <p className="p-sub">{p.contexte}</p>
            </div>
            <div className="p-actions">
              {combinee ? (
                <button className="boite-tag" title={'Ouvrir dans ' + p.boite}
                  onClick={() => ouvrirBoite(p.boite_id, '/suivi')}>{p.boite}</button>
              ) : (
                <>
                  <button className="btn-icon" aria-label="Marquer résolu" title="Marquer résolu" onClick={() => marquerLivre(p.engagement_id)}><Icone nom="etat-livre" /></button>
                  {p.action && (
                    <button className="btn-icon primary" aria-label={p.action.label} title={p.action.label}
                      onClick={() => d.openForEngagement(p.engagement_id, { ...p.action, hint: p.contexte })}><Icone nom="action-valider-envoyer" /></button>
                  )}
                </>
              )}
            </div>
          </div>
        ))}
      </div>

      {syn?.alertes?.length > 0 && (
        <>
          <div className="section-title">
            <h2>Alertes en cours</h2>
            <button className="lien-plus" onClick={() => navigate('/alertes')}>Toutes les alertes →</button>
          </div>
          <div className="priority-list">
            {syn.alertes.map((a) => (
              <div className={'priority-item' + (a.critique ? ' risque' : '')} key={(a.boite_id || 0) + '-' + a.id}>
                <span className={'p-badge' + (a.critique ? ' risque' : '')}>{DET_LABELS[a.type] || a.type}</span>
                <div className="p-body">
                  <p className="p-title">
                    {combinee
                      ? <button className="lien-source" title={'Ouvrir dans ' + a.boite}
                          onClick={() => ouvrirBoite(a.boite_id, '/alertes')}>{a.titre}</button>
                      : (a.engagement_id || a.thread_id)
                        ? <button className="lien-source" title="Voir la conversation d'origine"
                            onClick={() => setAlerteSrc(a)}>{a.titre}</button>
                        : a.titre}
                  </p>
                  <p className="p-sub">{a.detail}</p>
                </div>
                <div className="p-actions">
                  {combinee && <button className="boite-tag" title={'Ouvrir dans ' + a.boite}
                    onClick={() => ouvrirBoite(a.boite_id, '/alertes')}>{a.boite}</button>}
                  <Reli value={a.score} />
                </div>
              </div>
            ))}
          </div>
        </>
      )}

      <div className="section-title"><h2>Vue d'ensemble par catégorie</h2></div>
      <div className="cat-grid">
        {['encours', 'retard', 'risque'].map((cat) => {
          const bloc = cats[cat] || { nombre: 0, apercu: [] }
          const meta = CAT_META[cat]
          return (
            <div className={'cat-card ' + cat} key={cat}>
              <div className="cat-name"><span className={'dot ' + meta.dot} />{meta.titre}</div>
              <div className="cat-count">{bloc.nombre}</div>
              <div className="cat-preview">
                {(bloc.apercu || []).map((a, i) => <div className="cat-preview-item" key={i}>{a}</div>)}
                {!bloc.nombre && <div className="cat-preview-item">Aucun engagement dans cette catégorie.</div>}
              </div>
              {bloc.nombre > 0 && !combinee && (
                <button type="button" className="cat-link" onClick={() => navigate('/suivi?cat=' + cat)}>
                  Voir {bloc.nombre === 1 ? "l'engagement" : `les ${bloc.nombre} engagements`} →
                </button>
              )}
            </div>
          )
        })}
      </div>

      <div className="note">
        Une même cause racine n'est comptée qu'une fois : un retard fournisseur qui menace plusieurs
        engagements en aval génère une seule action de relance, affichée ici, plutôt qu'une alerte par
        engagement touché.
      </div>

      <DraftPanel draft={d.draft} loading={d.loading} title={d.meta.title} hint={d.meta.hint}
        onClose={d.close} onSent={d.onSent} />
      <SourcePanel engagementId={sourceId} onClose={() => setSourceId(null)} />
      {alerteSrc && (
        <SourcePanel
          engagementId={alerteSrc.engagement_id || undefined}
          threadId={alerteSrc.engagement_id ? undefined : alerteSrc.thread_id}
          onClose={() => setAlerteSrc(null)} />
      )}
    </section>
  )
}
