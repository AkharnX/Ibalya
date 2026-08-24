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

// Niveaux de fiabilité. Les seuils séparent réellement les données observées :
// sur les engagements extraits, 10 sont au-dessus de 85 %, 7 entre 70 et 80 %,
// et le reste sous 65 %. Le seuil de publication, lui, vaut 0,6 par défaut.
export const NIVEAUX_FIABILITE = [
  ['elevee', 'Élevée', 0.85, 1.01],
  ['moyenne', 'Moyenne', 0.65, 0.85],
  ['faible', 'Faible', 0, 0.65],
]

export const niveauFiabilite = (v) => {
  const x = v || 0
  return (NIVEAUX_FIABILITE.find(([, , min, max]) => x >= min && x < max) || NIVEAUX_FIABILITE[2])[0]
}

// Jauge de fiabilité. La barre seule ne disait pas combien : le pourcentage
// apparaît au survol, et au clavier — l'attribut title du navigateur met une
// seconde à s'afficher et ne se déclenche jamais au focus.
export const Reli = ({ value }) => {
  const pct = Math.round((value || 0) * 100)
  return (
    <div className={'reli ' + niveauFiabilite(value)} tabIndex={0}
      role="img" aria-label={`fiabilité ${pct} %`} data-pct={`${pct} %`}>
      <span style={{ width: `${pct}%` }} />
    </div>
  )
}

// Filtre par niveau de fiabilité, avec le compte de chaque niveau.
export const FiltreFiabilite = ({ valeur, onChange, rows, champ }) => {
  const comptes = { '': rows.length }
  rows.forEach((r) => {
    const n = niveauFiabilite(r[champ])
    comptes[n] = (comptes[n] || 0) + 1
  })
  return (
    <div className="chip-row">
      <div className={'chip' + (valeur === '' ? ' active' : '')} onClick={() => onChange('')}>
        Toutes fiabilités <span className="n">{comptes[''] || 0}</span>
      </div>
      {NIVEAUX_FIABILITE.map(([cle, label, min, max]) => (
        <div key={cle} className={'chip chip-' + cle + (valeur === cle ? ' active' : '')}
          onClick={() => onChange(cle)}
          title={max > 1 ? `${Math.round(min * 100)} % et plus` : `${Math.round(min * 100)} à ${Math.round(max * 100) - 1} %`}>
          {label} <span className="n">{comptes[cle] || 0}</span>
        </div>
      ))}
    </div>
  )
}

// Journal d'événements d'un engagement (CDC 5.2).
export const EVT_LABELS = {
  cree: 'Engagement détecté',
  confirme: 'Confirmé',
  signal_progression: 'Signe d’avancement',
  relance: 'Relance envoyée',
  livre: 'Livré',
  passe_en_retard: 'Passé en retard',
  abandonne: 'Écarté',
  corrige: 'Corrigé par le dirigeant',
}
