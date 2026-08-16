// Libellés et formats partagés entre les pages.
export const STATUT_LABELS = { ouvert: 'Ouvert', confirme: 'Confirmé', livre: 'Livré', en_retard: 'En retard', abandonne: 'Écarté' }
export const DET_LABELS = { echeance_a_risque: 'Échéance à risque', silence_anormal: 'Silence anormal', contradiction: 'Contradiction', orphelin: 'Orphelin', surcharge: 'Surcharge' }
export const TYPE_LABELS = {
  devis: 'Devis', livraison: 'Livraison', relance: 'Relance',
  prise_de_contact: 'Prise de contact', rendez_vous: 'Rendez-vous',
  facturation: 'Facturation', autre: 'Autre',
}

export const fmtDate = (s) => (s ? new Date(s).toLocaleDateString('fr-FR') : '—')
export const fmtDT = (s) => (s ? new Date(s).toLocaleString('fr-FR', { dateStyle: 'short', timeStyle: 'short' }) : '—')

export const Empty = ({ children }) => <div className="empty">{children}</div>
export const Reli = ({ value }) => (
  <div className="reli" title={`fiabilité ${Math.round(value * 100)} %`}>
    <span style={{ width: `${Math.round(value * 100)}%` }} />
  </div>
)
