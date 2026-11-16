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
  let table = null
  const fermerListe = () => { if (liste) { blocs.push(liste); liste = null } }
  const fermerTable = () => { if (table) { blocs.push(table); table = null } }
  ;(texte || '').split('\n').forEach((ligne, i) => {
    const l = ligne.trimEnd()
    // Tableau : lignes « | a | b | », la deuxième ligne « |---|---| » est le séparateur.
    if (/^\s*\|.*\|\s*$/.test(l)) {
      fermerListe()
      const cellules = l.trim().slice(1, -1).split('|').map((c) => c.trim())
      if (cellules.every((c) => /^:?-{2,}:?$/.test(c))) return
      if (!table) table = { entete: cellules, lignes: [] }
      else table.lignes.push(cellules)
      return
    }
    fermerTable()
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
  fermerTable()
  return (
    <div className="catalogue">
      {blocs.map((b, i) => {
        if (b.entete) {
          return (
            <div className="tbl-wrap" key={i}>
              <table>
                <thead><tr>{b.entete.map((c, k) => <th key={k}>{c}</th>)}</tr></thead>
                <tbody>{b.lignes.map((r, k) => <tr key={k}>{r.map((c, m) => <td key={m} className={m === 0 ? 'obj' : 'sub'}>{gras(c)}</td>)}</tr>)}</tbody>
              </table>
            </div>
          )
        }
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
  const [admin, setAdmin] = useState(undefined) // undefined = inconnu, false = refusé
  useEffect(() => {
    api('/me').then((me) => {
      setAdmin(!!me.admin)
      if (!me.admin) return
      api('/admin/fonctionnalites').then((r) => setCatalogue(r.markdown || '')).catch((e) => { setCatalogue(''); toast(e.message, true) })
      api('/admin/comptes').then((r) => setComptes(r || [])).catch((e) => { setComptes([]); toast(e.message, true) })
    }).catch(() => setAdmin(false))
  }, [])

  if (admin === false) {
    return (
      <section>
        <div className="page-head"><div><h1>Admin</h1></div></div>
        <div className="empty">Cette page est réservée aux administrateurs de l'instance.</div>
      </section>
    )
  }

  return (
    <section>
      <div className="page-head">
        <div>
          <h1>Admin</h1>
          <p>Les comptes de l'instance avec leurs boîtes, et le catalogue des fonctionnalités tel qu'il est dans le dépôt. La connexion est l'ouverture de session (valable 30 jours) ; l'activité, la dernière fois que le compte a utilisé l'application.</p>
        </div>
      </div>

      <div className="section-title"><h2>Comptes</h2></div>
      {comptes === undefined && <SqueletteLignes nombre={3} />}
      {comptes && (
        <div className="tbl-wrap">
          <table>
            <thead>
              <tr><th>Compte</th><th>Adresse</th><th>Boîtes</th><th>Créé le</th><th>Dernière connexion</th><th>Dernière activité</th><th>État</th></tr>
            </thead>
            <tbody>
              {comptes.map((u) => (
                <tr key={u.id}>
                  <td className="obj">{u.nom || '–'}</td>
                  <td className="sub mono">{u.email || '–'}</td>
                  <td className="sub">{(u.boites || []).length ? u.boites.join(', ') : 'aucune'}</td>
                  <td className="sub">{fmtDT(u.cree_le)}</td>
                  <td className="sub">{u.derniere_connexion ? fmtDT(u.derniere_connexion) : 'jamais'}</td>
                  <td className="sub">{u.derniere_activite ? fmtDT(u.derniere_activite) : 'jamais'}</td>
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
