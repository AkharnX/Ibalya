import { useEffect, useState } from 'react'
import { api, toast } from '../api'
import { fmtDT } from '../components/ui'
import { SqueletteLignes } from '../components/Squelette'

// Page Admin : le catalogue des fonctionnalités (docs/FONCTIONNALITES.md, servi
// tel quel par le backend) et la liste des comptes. Visible seulement pour les
// adresses listées dans ADMIN_EMAILS.

// Rendu minimal du Markdown du catalogue : titres, listes, gras, paragraphes.
// Le document est le nôtre, pas du contenu tiers : pas de HTML brut interprété.
function gras(texte) {
  const parts = texte.split(/(\*\*[^*]+\*\*|`[^`]+`)/g)
  return parts.map((p, i) => {
    if (p.startsWith('**') && p.endsWith('**')) return <b key={i}>{p.slice(2, -2)}</b>
    if (p.startsWith('`') && p.endsWith('`')) return <code key={i}>{p.slice(1, -1)}</code>
    return p
  })
}

function Markdown({ texte }) {
  const blocs = []
  let liste = null
  const fermerListe = () => { if (liste) { blocs.push(liste); liste = null } }
  ;(texte || '').split('\n').forEach((ligne, i) => {
    const l = ligne.trimEnd()
    const puce = l.match(/^(\s*)([-*]|\d+\.)\s+(.*)$/)
    if (puce) {
      const ordonnee = /\d+\./.test(puce[2])
      if (!liste || liste.ordonnee !== ordonnee) { fermerListe(); liste = { ordonnee, items: [] } }
      liste.items.push(puce[3])
      return
    }
    fermerListe()
    if (!l) return
    const titre = l.match(/^(#{1,3})\s+(.*)$/)
    if (titre) { blocs.push({ niveau: titre[1].length, texte: titre[2], key: i }); return }
    blocs.push({ p: l, key: i })
  })
  fermerListe()
  return (
    <div className="catalogue">
      {blocs.map((b, i) => {
        if (b.items) {
          const Tag = b.ordonnee ? 'ol' : 'ul'
          return <Tag key={i}>{b.items.map((it, k) => <li key={k}>{gras(it)}</li>)}</Tag>
        }
        if (b.niveau === 1) return <h1 key={i}>{b.texte}</h1>
        if (b.niveau === 2) return <h2 key={i}>{b.texte}</h2>
        if (b.niveau === 3) return <h3 key={i}>{b.texte}</h3>
        return <p key={i}>{gras(b.p)}</p>
      })}
    </div>
  )
}

export default function Admin() {
  const [catalogue, setCatalogue] = useState(undefined)
  const [comptes, setComptes] = useState(undefined)
  useEffect(() => {
    api('/admin/fonctionnalites').then((r) => setCatalogue(r.markdown || '')).catch((e) => { setCatalogue(''); toast(e.message, true) })
    api('/admin/comptes').then((r) => setComptes(r || [])).catch((e) => { setComptes([]); toast(e.message, true) })
  }, [])

  return (
    <section>
      <div className="page-head">
        <div>
          <h1>Admin</h1>
          <p>Le catalogue des fonctionnalités tel qu'il est dans le dépôt, et les comptes de l'instance.</p>
        </div>
      </div>

      <div className="section-title"><h2>Comptes</h2></div>
      {comptes === undefined && <SqueletteLignes nombre={3} />}
      {comptes && (
        <div className="tbl-wrap">
          <table>
            <thead>
              <tr><th>Boîte</th><th>Adresse</th><th>Créé le</th><th>Dernière connexion</th><th>État</th></tr>
            </thead>
            <tbody>
              {comptes.map((u) => (
                <tr key={u.id}>
                  <td className="obj">{u.libelle || u.nom || '–'}</td>
                  <td className="sub mono">{u.email || '–'}</td>
                  <td className="sub">{fmtDT(u.cree_le)}</td>
                  <td className="sub">{u.derniere_connexion ? fmtDT(u.derniere_connexion) : 'jamais'}</td>
                  <td className="sub">{u.actif ? 'actif' : 'désactivé'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <div className="section-title"><h2>Fonctionnalités</h2></div>
      {catalogue === undefined && <SqueletteLignes nombre={6} />}
      {catalogue === '' && <div className="empty">Catalogue introuvable sur le serveur (docs/FONCTIONNALITES.md).</div>}
      {catalogue && <div className="panel"><Markdown texte={catalogue} /></div>}
    </section>
  )
}
