// Tri de tableau réutilisable : un clic sur un en-tête trie, un second inverse.
// Usage :
//   const { tri, trierPar, trier } = useTri()           // ou useTri('date','desc')
//   const lignes = trier(donnees, { col: (r) => valeur })
//   <EnteteTri col="col" tri={tri} trierPar={trierPar}>Libellé</EnteteTri>
import { useState } from 'react'

export function useTri(colDefaut = null, sensDefaut = 'asc') {
  const [tri, setTri] = useState({ col: colDefaut, sens: sensDefaut })
  const trierPar = (col) =>
    setTri((t) => (t.col === col ? { col, sens: t.sens === 'asc' ? 'desc' : 'asc' } : { col, sens: 'asc' }))
  const trier = (donnees, cles) => {
    const data = donnees || []
    const f = tri.col && cles[tri.col]
    if (!f) return data
    return [...data].sort((a, b) => {
      const va = f(a), vb = f(b)
      const c = va < vb ? -1 : va > vb ? 1 : 0
      return tri.sens === 'asc' ? c : -c
    })
  }
  return { tri, trierPar, trier }
}

export function EnteteTri({ col, tri, trierPar, children, ...rest }) {
  const actif = tri.col === col
  return (
    <th aria-sort={actif ? (tri.sens === 'asc' ? 'ascending' : 'descending') : 'none'} {...rest}>
      <button type="button" className={'th-tri' + (actif ? ' actif' : '')} onClick={() => trierPar(col)}>
        {children}
        <span className="th-fleche" aria-hidden="true">{actif ? (tri.sens === 'asc' ? '↑' : '↓') : '↕'}</span>
      </button>
    </th>
  )
}
