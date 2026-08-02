// Client API — jeton en localStorage, erreurs normalisées.
let token = localStorage.getItem('agentops_token') || '';

export const getToken = () => token;
export function setToken(t) {
  token = t;
  localStorage.setItem('agentops_token', t);
}

export class AuthError extends Error {}

export async function api(path, opts = {}) {
  const resp = await fetch('/api' + path, {
    ...opts,
    headers: { Authorization: 'Bearer ' + token, 'Content-Type': 'application/json', ...(opts.headers || {}) },
  });
  if (resp.status === 401) throw new AuthError('Jeton invalide');
  const data = await resp.json().catch(() => ({}));
  if (!resp.ok) throw new Error(data.error || resp.statusText);
  return data;
}

// Toast minimaliste : dispatch d'un événement, écouté par <Toaster/>.
export function toast(message, isError = false) {
  window.dispatchEvent(new CustomEvent('agentops:toast', { detail: { message, isError } }));
}
