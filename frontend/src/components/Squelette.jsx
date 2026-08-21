// Squelettes de chargement : montrer la forme du contenu à venir plutôt qu'un
// écran vide. L'attente paraît plus courte à durée égale, et l'utilisateur sait
// que quelque chose arrive.
export function SqueletteTable({ lignes = 5, colonnes = 5 }) {
  return (
    <div className="tbl-wrap" aria-busy="true" aria-label="Chargement">
      <table>
        <tbody>
          {Array.from({ length: lignes }).map((_, i) => (
            <tr key={i}>
              {Array.from({ length: colonnes }).map((_, j) => (
                <td key={j}>
                  <span className="sq" style={{
                    width: j === 1 ? '78%' : j === 0 ? '64px' : '46%',
                    animationDelay: `${(i * colonnes + j) * 40}ms`,
                  }} />
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

export function SqueletteKpi({ nombre = 5 }) {
  return (
    <div className="kpi-row" aria-busy="true">
      {Array.from({ length: nombre }).map((_, i) => (
        <div className="kpi static" key={i}>
          <span className="sq sq-court" style={{ animationDelay: `${i * 60}ms` }} />
          <span className="sq sq-grand" style={{ animationDelay: `${i * 60 + 30}ms` }} />
        </div>
      ))}
    </div>
  )
}

export function SqueletteLignes({ nombre = 3 }) {
  return (
    <div aria-busy="true">
      {Array.from({ length: nombre }).map((_, i) => (
        <div className="item" key={i}>
          <span className="sq" style={{ width: '38%', animationDelay: `${i * 60}ms` }} />
          <span className="sq sq-fin" style={{ width: '72%', animationDelay: `${i * 60 + 30}ms` }} />
        </div>
      ))}
    </div>
  )
}
