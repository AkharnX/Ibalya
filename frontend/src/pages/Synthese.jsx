import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, toast } from '../api'
import { DraftPanel, useDraft } from '../components/DraftPanel'

const CAT_META = {
  encours: { dot: 'blue', titre: 'Engagements en cours' },
  retard: { dot: 'amber', titre: 'Retards probables' },
  risque: { dot: 'red', titre: 'Engagements à risque' },
}

export default function Synthese() {
  const [syn, setSyn] = useState(null)
  const navigate = useNavigate()

  const load = useCallback(() => {
    api('/synthese').then(setSyn).catch((e) => toast(e.message, true))
  }, [])
  useEffect(load, [load])

  const d = useDraft(load)

  const runCycle = async () => {
    toast('Analyse en cours…')
    try {
      const r = await api('/cycle/run', { method: 'POST', body: JSON.stringify({ since_days: 2 }) })
      toast(r.erreur ? 'Terminé avec erreur : ' + r.erreur : `Analyse terminée (${r.duree})`, !!r.erreur)
      load()
    } catch (e) { toast(e.message, true) }
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
          <h1>Synthèse</h1>
          <p>L'essentiel de votre activité en un coup d'œil : ce qui bloque, ce qui arrive, ce que vous pouvez traiter sans quitter cette page.</p>
        </div>
        <div className="page-actions">
          <button className="btn" onClick={runCycle}>⟳ Analyser</button>
        </div>
      </div>

      <div className="kpi-row">
        <button className="kpi" onClick={() => navigate('/suivi')}>
          <span className="lbl">Actifs</span><span className="num">{k?.engagements_suivis ?? '—'}</span>
        </button>
        <button className="kpi warn" onClick={() => navigate('/suivi?cat=retard')}>
          <span className="lbl">En retard</span><span className="num">{k?.retards ?? '—'}</span>
        </button>
        <button className="kpi risk flag" onClick={() => navigate('/suivi?cat=risque')}>
          <span className="lbl">Critiques</span><span className="num">{k?.risques ?? '—'}</span>
        </button>
        <button className="kpi accent flag" onClick={() => navigate('/a-valider')}>
          <span className="lbl">À valider</span><span className="num">{k?.messages_a_valider ?? '—'}</span>
        </button>
        <div className="kpi static">
          <span className="lbl">Messages lus / 30j</span><span className="num">{k?.messages_lus ?? '—'}</span>
        </div>
      </div>

      <div className="section-title"><h2>Ce qui demande une décision maintenant</h2></div>
      <div className="priority-list">
        {!syn?.priorites?.length && <div className="empty">Rien à arbitrer — aucun retard ni engagement bloqué. 👌</div>}
        {(syn?.priorites || []).map((p) => (
          <div className={'priority-item ' + p.categorie} key={p.engagement_id}>
            <span className={'p-badge ' + p.categorie}>{p.categorie === 'risque' ? 'À risque' : 'Retard'}</span>
            <div className="p-body">
              <p className="p-title">{p.titre}</p>
              <p className="p-sub">{p.contexte}</p>
            </div>
            <div className="p-actions">
              <button className="btn-icon" title="Marquer résolu" onClick={() => marquerLivre(p.engagement_id)}>✓</button>
              {p.action && (
                <button className="btn-icon primary" title={p.action.label}
                  onClick={() => d.openForEngagement(p.engagement_id, { ...p.action, hint: p.contexte })}>✉</button>
              )}
            </div>
          </div>
        ))}
      </div>

      <div className="section-title"><h2>Vue d'ensemble par catégorie</h2></div>
      <div className="cat-grid">
        {['encours', 'retard', 'risque'].map((cat) => {
          const bloc = cats[cat] || { nombre: 0, apercu: [] }
          const meta = CAT_META[cat]
          return (
            <div className={'cat-card ' + cat} key={cat}>
              <div className="cat-head">
                <div className="cat-name"><span className={'dot ' + meta.dot} />{meta.titre}</div>
              </div>
              <div className="cat-count">{bloc.nombre}</div>
              <div className="cat-preview">
                {(bloc.apercu || []).map((a, i) => <div className="cat-preview-item" key={i}>{a}</div>)}
                {!bloc.nombre && <div className="cat-preview-item">Aucun engagement dans cette catégorie.</div>}
              </div>
              {bloc.nombre > 0 && (
                <div className="cat-link" onClick={() => navigate('/suivi?cat=' + cat)}>
                  Voir {bloc.nombre === 1 ? "l'engagement" : `les ${bloc.nombre} engagements`} →
                </div>
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
    </section>
  )
}
