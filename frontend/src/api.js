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

// Toast minimaliste : dispatch d'un événement, écouté par <Toaster/>.
export function toast(message, isError = false) {
  window.dispatchEvent(new CustomEvent('ibalya:toast', { detail: { message, isError } }));
}
