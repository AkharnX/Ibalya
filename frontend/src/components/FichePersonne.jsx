// Fiche d'un interlocuteur.
//
// La page listait les personnes déduites des échanges sans permettre d'en
// ouvrir une. C'est pourtant là que se pose la question utile du CDC — « où en
// est-on avec ce client » — dont la réponse était dispersée entre le suivi des
// engagements et les conversations.
import { useEffect, useState } from 'react'
import Icone from './Icone'
import { api } from '../api'
import { Reli, STATUT_LABELS, TYPE_LABELS, fmtDate, fmtDT } from './ui'

const TYPE_PERSONNE = { interne: 'Interne', client: 'Client', fournisseur: 'Fournisseur', autre: 'Non classé' }

export default function FichePersonne({ personneId, onClose, onEngagement, onFil }) {
  const [f, setF] = useState(null)
  const [erreur, setErreur] = useState('')

  useEffect(() => {
    if (!personneId) { setF(null); setErreur(''); return }
    setF(null); setErreur('')
    api(`/persons/${personneId}`).then(setF).catch((e) => setErreur(e.message))
  }, [personneId])

  const ouvert = !!personneId

  return (
    <>
      <div className={'overlay' + (ouvert ? ' open' : '')} onClick={onClose} />
      <div className={'draft-panel source-panel' + (ouvert ? ' open' : '')}>
        <div className="draft-head">
          <div>
            <h3>{f?.name || f?.email || 'Interlocuteur'}</h3>
            <p>{f ? f.email : 'Chargement…'}</p>
          </div>
          <button className="draft-close" title="Fermer" onClick={onClose}><Icone nom="action-fermer-panneau" /></button>
        </div>

        <div className="draft-body">
          {erreur && <div className="empty">{erreur}</div>}
          {f && (
            <>
              <div className="fiche-etiquettes">
                <span className="tag">{TYPE_PERSONNE[f.type] || 'Non classé'}</span>
                {f.sensitive && <span className="tag tag-vigilance">Vigilance accrue</span>}
              </div>

              <div className="fiche-chiffres">
                <div><b>{f.messages_echanges}</b><span>messages échangés</span></div>
                <div><b>{f.en_cours}</b><span>en cours</span></div>
                <div><b>{f.en_retard}</b><span>en retard</span></div>
                <div><b>{f.livres}</b><span>livrés</span></div>
              </div>
              <p className="help">
                {f.dernier_echange
                  ? `Dernier échange le ${fmtDT(f.dernier_echange)}.`
                  : 'Aucun échange conservé pour cette adresse.'}
              </p>

              <h4>Engagements</h4>
              {!f.engagements.length ? (
                <p className="help">Aucun engagement extrait avec cette personne.</p>
              ) : (
                <ul className="fiche-liste">
                  {f.engagements.map((e) => (
                    <li key={e.id}>
                      <button className="lien-source" onClick={() => onEngagement?.(e.id)}>{e.objet}</button>
                      <div className="fiche-meta">
                        <span className={'badge ' + (e.type || 'autre')}>{TYPE_LABELS[e.type] || 'Autre'}</span>
                        <span className="sub">{STATUT_LABELS[e.statut] || e.statut}</span>
                        <span className="sub">{e.echeance ? fmtDate(e.echeance) : 'sans échéance'}</span>
                        <Reli value={e.confiance} />
                      </div>
                    </li>
                  ))}
                </ul>
              )}

              <h4>Conversations</h4>
              {!f.fils.length ? (
                <p className="help">Aucune conversation conservée.</p>
              ) : (
                <ul className="fiche-liste">
                  {f.fils.map((t) => (
                    <li key={t.id}>
                      <button className="lien-source" onClick={() => onFil?.(t.id)}>
                        {t.subject || '(sans objet)'}
                      </button>
                      <div className="fiche-meta">
                        <span className="sub">
                          {t.last_message_at ? `dernier message le ${fmtDate(t.last_message_at)}` : 'date inconnue'}
                        </span>
                        {t.excluded && <span className="sub">écarté du suivi</span>}
                      </div>
                    </li>
                  ))}
                </ul>
              )}
            </>
          )}
        </div>
      </div>
    </>
  )
}
