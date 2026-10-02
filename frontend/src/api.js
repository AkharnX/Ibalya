// Client API — authentification par session (cookie HttpOnly posé par le serveur).
// Aucun jeton n'est stocké côté navigateur : rien à voler via un XSS.

export class AuthError extends Error {}

export async function api(path, opts = {}) {
  const resp = await fetch('/api' + path, {
    credentials: 'same-origin',
    ...opts,
    headers: { 'Content-Type': 'application/json', ...(opts.headers || {}) },
  });
  if (resp.status === 401) throw new AuthError('Session expirée');
  const data = await resp.json().catch(() => ({}));
  if (!resp.ok) throw new Error(data.error || resp.statusText);
  return data;
}

export const login = (email, motDePasse) =>
  api('/login', { method: 'POST', body: JSON.stringify({ email, mot_de_passe: motDePasse }) });

export const logout = () => api('/logout', { method: 'POST' });

// Boîtes (espaces) du compte connecté.
export const espaces = () => api('/espaces');
export const creerEspace = (libelle) =>
  api('/espaces', { method: 'POST', body: JSON.stringify({ libelle }) });
export const activerEspace = (id) => api(`/espaces/${id}/activer`, { method: 'POST' });
export const supprimerEspace = (id) => api(`/espaces/${id}`, { method: 'DELETE' });

// Vue combinée « toutes les boîtes » : préférence par navigateur (pas d'état
// serveur, l'espace actif de la session reste inchangé). Les écrans combinés
// lisent les agrégats /synthese/global et /drafts/global.
export const CLE_VUE_COMBINEE = 'ibalya_vue_combinee'
export const estVueCombinee = () => {
  try { return localStorage.getItem(CLE_VUE_COMBINEE) === '1' } catch { return false }
}
export const definirVueCombinee = (actif) => {
  try { actif ? localStorage.setItem(CLE_VUE_COMBINEE, '1') : localStorage.removeItem(CLE_VUE_COMBINEE) } catch { /* stockage indisponible */ }
}

// Toast minimaliste : dispatch d'un événement, écouté par <Toaster/>.
export function toast(message, isError = false) {
  window.dispatchEvent(new CustomEvent('ibalya:toast', { detail: { message, isError } }));
}
