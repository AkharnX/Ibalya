// Éditeur de liste sous forme d'étiquettes : ajouter, retirer, sans JSON.
// Tolère les entrées textuelles simples comme les objets {nom, role, email}
// renvoyés par le modèle.
import { useState } from 'react'

export const entryLabel = (v) => {
  if (typeof v === 'string') return v
  if (v && typeof v === 'object') {
    const nom = v.nom || v.name || ''
    const email = v.email || ''
    if (nom && email) return `${nom} — ${email}`
    return nom || email || JSON.stringify(v)
  }
  return String(v ?? '')
}

export default function ListEditor({ label, hint, value, onChange, placeholder }) {
  const [draft, setDraft] = useState('')
  const items = Array.isArray(value) ? value : []

  const add = () => {
    const v = draft.trim()
    if (!v) return
    onChange([...items, v])
    setDraft('')
  }
  const remove = (i) => onChange(items.filter((_, idx) => idx !== i))

  return (
    <div className="field">
      <label>{label}</label>
      {hint && <p className="field-hint">{hint}</p>}
      <div className="tag-list">
        {items.map((it, i) => (
          <span className="tag-edit" key={i}>
            {entryLabel(it)}
            <button type="button" title="Retirer" onClick={() => remove(i)}>×</button>
          </span>
        ))}
        {!items.length && <span className="field-hint">Aucun pour l'instant.</span>}
      </div>
      <div className="tag-add">
        <input
          value={draft} placeholder={placeholder} onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); add() } }}
        />
        <button type="button" onClick={add}>Ajouter</button>
      </div>
    </div>
  )
}
