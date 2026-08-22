// Sorti du HTML pour qu'aucun script en ligne ne subsiste : la politique de
// sécurité du contenu peut alors interdire l'exécution de scripts injectés.
document.getElementById('lp-annee').textContent = new Date().getFullYear()
