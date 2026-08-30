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

// Niveaux de fiabilité, alignés sur les seuils du back-end.
//
// Les seuils précédents (0,85 et 0,65) étaient calés sur une distribution que
// le modèle produisait lui-même en notant sa propre certitude : elle tassait
// tout au-dessus de 0,90 et ne séparait rien. Le score est désormais calculé
// à partir de signaux vérifiables, et ces seuils sont ceux d'engine/fiabilite.go.
export const NIVEAUX_FIABILITE = [
  ['elevee', 'Élevée', 0.75, 1.01],
  ['a_verifier', 'À vérifier', 0.50, 0.75],
  ['incertaine', 'Incertaine', 0, 0.50],
]

export const niveauFiabilite = (v) => {
  const x = v || 0
  return (NIVEAUX_FIABILITE.find(([, , min, max]) => x >= min && x < max) || NIVEAUX_FIABILITE[2])[0]
}

// Jauge de fiabilité.
//
// Elle affichait le score en pourcentage. « 92 % » annonce une mesure calibrée,
// que ce score n'est pas : il combine cinq signaux pondérés à la main, et rien
// ne garantit encore que 0,92 se trompe deux fois moins que 0,84. Le niveau
// dit ce qu'on sait sans promettre ce qu'on ignore.
export const Reli = ({ value }) => {
  const niveau = niveauFiabilite(value)
  const libelle = (NIVEAUX_FIABILITE.find(([c]) => c === niveau) || [])[1] || ''
  const pct = Math.round((value || 0) * 100)
  return (
    <div className={'reli ' + niveau} tabIndex={0}
      role="img" aria-label={`fiabilité : ${libelle}`} data-pct={libelle}>
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
