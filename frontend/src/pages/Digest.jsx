import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, toast } from '../api'
import { DET_LABELS, Reli, TYPE_LABELS, fmtDT, fmtDate } from '../components/ui'
import { SqueletteLignes } from '../components/Squelette'

// Digest (CDC 9.3, EF-6). C'est un document figé, pas une vue vivante : chaque
// édition est le récapitulatif tel qu'il a été produit et envoyé ce jour-là.
// Les mêmes alertes se retrouvent sur « Alertes » et les mêmes messages sur
// « À valider » — c'est là qu'on agit, pas ici.
const libelleType = (t) => (t === 'digest_hebdo' ? 'Hebdomadaire' : 'Quotidien')

export default function Digest() {
  const [editions, setEditions] = useState(null)
  const [courante, setCourante] = useState(null)   // id sélectionné
  const [rep, setRep] = useState(undefined)
  const [reglages, setReglages] = useState(null)
  const [compte, setCompte] = useState('')
  const [busy, setBusy] = useState(false)

  const chargerEditions = useCallback((selectionner) => {
    api('/digest/editions').then((r) => {
      const liste = r || []
      setEditions(liste)
      const id = selectionner ?? liste[0]?.id ?? null
      setCourante(id)
      if (id === null) setRep(null)
    }).catch((e) => { setEditions([]); setRep(null); toast(e.message, true) })
  }, [])

  useEffect(() => { chargerEditions() }, [chargerEditions])
  useEffect(() => {
    api('/settings').then(setReglages).catch(() => {})
    api('/status').then((s) => setCompte(s.compte || '')).catch(() => {})
  }, [])

  useEffect(() => {
    if (courante === null) return
    setRep(undefined)
    api(`/digest/${courante}`).then(setRep).catch((e) => { setRep(null); toast(e.message, true) })
  }, [courante])

  const generer = async () => {
    if (busy) return
    setBusy(true)
    try {
      await api('/digest/generate', { method: 'POST', body: JSON.stringify({ type: reglages?.digest_type || 'quotidien' }) })
      toast('Nouvelle édition produite')
      chargerEditions(null)
    } catch (e) { toast(e.message, true) }
    finally { setBusy(false) }
  }

  const d = rep?.content
  const envoiActif = reglages?.digest_email === '1'
  const nb = (k) => (d?.[k] || []).length

  return (
    <section>
      <div className="page-head">
        <div>
          <h1>Digest</h1>
          <p>Les récapitulatifs que l'agent a produits, édition par édition. Chaque document est figé tel qu'il a été envoyé : c'est l'archive, pas la vue du moment.</p>
        </div>
        <div className="page-actions">
          <button className="btn" disabled={busy} onClick={generer}>{busy ? 'Production…' : '⟳ Produire une édition'}</button>
        </div>
      </div>

      {editions === null && <SqueletteLignes nombre={3} />}

      {editions && !editions.length && (
        <div className="empty">Aucune édition n'a encore été produite.</div>
      )}

      {editions && editions.length > 0 && (
        <>
          <div className="edition-bar">
            <label htmlFor="edition">Édition</label>
            <select id="edition" value={courante ?? ''} onChange={(e) => setCourante(Number(e.target.value))}>
              {editions.map((e) => (
                <option key={e.id} value={e.id}>
                  {fmtDate(e.created_at)} — {libelleType(e.type)}
                </option>
              ))}
            </select>
            <span className="edition-total">
              {editions.length} édition{editions.length > 1 ? 's' : ''} archivée{editions.length > 1 ? 's' : ''}
            </span>
          </div>

          {rep === undefined && <SqueletteLignes nombre={4} />}
          {rep === null && <div className="empty">Cette édition est introuvable.</div>}

          {d && (
            <article className="document">
              <header className="document-head">
                <h2>Digest {libelleType(rep.type).toLowerCase()} du {fmtDate(rep.created_at)}</h2>
                <p className="sub">
                  Produit le {fmtDT(d.genere_le || rep.created_at)}
                  {envoiActif && compte ? ` · envoyé à ${compte}` : ' · envoi par email désactivé'}
                </p>
                <div className="document-resume">
                  <span><b>{nb('detections')}</b> alerte{nb('detections') > 1 ? 's' : ''} retenue{nb('detections') > 1 ? 's' : ''}</span>
                  <span><b>{nb('engagements_a_risque')}</b> engagement{nb('engagements_a_risque') > 1 ? 's' : ''} à risque</span>
                  <span><b>{nb('brouillons_proposes')}</b> message{nb('brouillons_proposes') > 1 ? 's' : ''} proposé{nb('brouillons_proposes') > 1 ? 's' : ''}</span>
                </div>
              </header>

              <h3>Alertes retenues</h3>
              {!nb('detections') ? <p className="help">Aucune alerte au-dessus du seuil de fiabilité ce jour-là.</p> : (
                <ul className="document-liste">
                  {d.detections.map((x) => (
                    <li key={x.id}>
                      <span className="doc-etiquette">{x.critique ? '⚠ ' : ''}{DET_LABELS[x.type] || x.type}</span>
                      <span className="doc-texte">{x.titre}{x.detail ? ` — ${x.detail}` : ''}</span>
                      <Reli value={x.score} />
                    </li>
                  ))}
                </ul>
              )}

              <h3>Engagements à risque</h3>
              {!nb('engagements_a_risque') ? <p className="help">Aucun engagement à risque ce jour-là.</p> : (
                <ul className="document-liste">
                  {d.engagements_a_risque.map((e) => (
                    <li key={e.id}>
                      <span className="doc-etiquette">{TYPE_LABELS[e.type] || 'Autre'}</span>
                      <span className="doc-texte">
                        {e.objet}
                        <em className="sub"> · {e.emetteur_email || '?'} → {e.destinataire_email || '?'}</em>
                      </span>
                      <span className="mono">{e.echeance ? fmtDate(e.echeance) : '—'}</span>
                    </li>
                  ))}
                </ul>
              )}

              <h3>Messages proposés</h3>
              {!nb('brouillons_proposes') ? <p className="help">Aucun message proposé ce jour-là.</p> : (
                <ul className="document-liste">
                  {d.brouillons_proposes.map((b) => (
                    <li key={b.id}>
                      <span className="doc-etiquette">{b.to_email}</span>
                      <span className="doc-texte">{b.subject}</span>
                    </li>
                  ))}
                </ul>
              )}

              <footer className="document-pied">
                Document figé, tel qu'il a été produit. Pour traiter ces éléments, allez sur{' '}
                <Link to="/alertes">Alertes</Link> ou <Link to="/a-valider">À valider</Link>.
              </footer>
            </article>
          )}
        </>
      )}
    </section>
  )
}
