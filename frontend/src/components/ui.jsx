// Composants partagés : badges, tables, tuiles.
import { api, toast } from '../api'

export const STATUT_LABELS = { ouvert: 'Ouvert', confirme: 'Confirmé', livre: 'Livré', en_retard: 'En retard', abandonne: 'Écarté' }
export const DET_LABELS = { echeance_a_risque: 'Échéance à risque', silence_anormal: 'Silence anormal', contradiction: 'Contradiction', orphelin: 'Orphelin', surcharge: 'Surcharge' }
export const TYPE_LABELS = { devis: 'Devis', livraison: 'Livraison', relance: 'Relance', prise_de_contact: 'Prise de contact', rendez_vous: 'Rendez-vous', facturation: 'Facturation', autre: 'Autre' }

export const fmtDate = (s) => (s ? new Date(s).toLocaleDateString('fr-FR') : '—')
export const fmtDT = (s) => (s ? new Date(s).toLocaleString('fr-FR', { dateStyle: 'short', timeStyle: 'short' }) : '—')

export const TypeBadge = ({ type }) => <span className={`badge b-${type || 'autre'}`}>{TYPE_LABELS[type] || 'Autre'}</span>
export const StatutPill = ({ statut }) => <span className={`st st-${statut}`}>{STATUT_LABELS[statut] || statut}</span>
export const ConfBar = ({ value }) => (
  <span className="conf" title={`fiabilité ${(value * 100).toFixed(0)} %`}>
    <i style={{ width: `${(value * 100).toFixed(0)}%` }} />
  </span>
)
export const Empty = ({ children }) => <p className="empty">{children}</p>

// Actions communes sur un engagement (boucle d'apprentissage).
export async function engAction(fn, refresh) {
  try {
    const r = await fn()
    toast(r?.regle ? 'Règle apprise : ' + r.regle : 'Corrigé')
    refresh?.()
  } catch (e) {
    toast(e.message, true)
  }
}

export const patchEng = (id, body, refresh) => engAction(() => api(`/engagements/${id}`, { method: 'PATCH', body: JSON.stringify(body) }), refresh)
export const correct = (id, action, refresh) => engAction(() => api(`/engagements/${id}/correct`, { method: 'POST', body: JSON.stringify({ action }) }), refresh)
export function confirmEcheance(id, current, refresh) {
  const d = prompt("Confirmer ou corriger l'échéance (AAAA-MM-JJ) :", current)
  if (d) patchEng(id, { echeance: d }, refresh)
}

function EcheanceCell({ e }) {
  if (!e.echeance) return <span className="muted">—</span>
  return (
    <>
      {fmtDate(e.echeance)}
      {e.echeance_inferee && !e.echeance_confirmee && <> <span className="tag warn-tag">à confirmer</span></>}
    </>
  )
}

export function EngTable({ engs, compact = false, refresh, emptyText = 'Aucun engagement.' }) {
  if (!engs || !engs.length) return <Empty>{emptyText}</Empty>
  return (
    <div className="tbl-wrap">
      <table className="tbl">
        <thead>
          <tr>
            <th>Type</th><th>Engagement</th><th>Échéance</th><th>Fiabilité</th><th>Statut</th>
            {!compact && <th></th>}
          </tr>
        </thead>
        <tbody>
          {engs.map((e) => (
            <tr key={e.id}>
              <td><TypeBadge type={e.type} /></td>
              <td className="obj">
                {e.objet} {e.priorite === 'haute' && <span className="tag warn-tag">prioritaire</span>}
                <div className="sub">{e.emetteur_email || '?'} → {e.destinataire_email || '?'}</div>
              </td>
              <td><EcheanceCell e={e} /></td>
              <td><ConfBar value={e.confiance} /></td>
              <td><StatutPill statut={e.statut} /></td>
              {!compact && (
                <td>
                  <div className="row-actions">
                    <button className="ghost" title="Marquer livré" onClick={() => patchEng(e.id, { statut: 'livre' }, refresh)}>✓</button>
                    <button className="ghost" title="Pas un engagement" onClick={() => correct(e.id, 'pas_un_engagement', refresh)}>✗</button>
                    <button className="ghost" title="Priorité haute" onClick={() => correct(e.id, 'priorite_haute', refresh)}>↑</button>
                    <button className="ghost" title="Ne plus alerter sur ce fil" onClick={() => correct(e.id, 'ne_plus_alerter', refresh)}>🔕</button>
                    {e.echeance && !e.echeance_confirmee && (
                      <button className="ghost" title="Confirmer l'échéance" onClick={() => confirmEcheance(e.id, (e.echeance || '').slice(0, 10), refresh)}>📅</button>
                    )}
                  </div>
                </td>
              )}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
