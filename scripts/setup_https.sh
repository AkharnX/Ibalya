#!/usr/bin/env bash
# Met Ibalya en ligne sur https://ibalya.com.
# À lancer UNE FOIS, après avoir fait pointer les enregistrements DNS A vers ce serveur.
set -euo pipefail
cd "$(dirname "$0")/.."

DOMAINE="${DOMAINE:-ibalya.com}"
IP_SERVEUR="${IP_SERVEUR:-157.180.36.122}"
EMAIL="${EMAIL:-ibkebe2002@gmail.com}"

etape() { echo; echo "━━━ $1 ━━━"; }

etape "1. Vérification du DNS"
for h in "$DOMAINE" "www.$DOMAINE"; do
  ip=$(dig +short "$h" A @1.1.1.1 | tail -1)
  if [ "$ip" != "$IP_SERVEUR" ]; then
    echo "  ✗ $h pointe vers ${ip:-rien} au lieu de $IP_SERVEUR"
    echo "    Corrigez la zone DNS chez OVH, puis relancez (propagation : quelques minutes)."
    exit 1
  fi
  echo "  ✓ $h → $ip"
done

etape "2. Installation du vhost nginx"
sudo cp deploy/nginx-$DOMAINE.conf /etc/nginx/sites-available/$DOMAINE
sudo ln -sf /etc/nginx/sites-available/$DOMAINE /etc/nginx/sites-enabled/$DOMAINE
sudo nginx -t
sudo systemctl reload nginx
echo "  ✓ vhost actif en HTTP"

etape "3. Certificat Let's Encrypt"
sudo certbot --nginx -d "$DOMAINE" -d "www.$DOMAINE" \
  --non-interactive --agree-tos -m "$EMAIL" --redirect
echo "  ✓ certificat installé, redirection HTTP → HTTPS activée"

etape "4. Bascule de l'application sur le domaine"
sed -i "s|^PUBLIC_BASE_URL=.*|PUBLIC_BASE_URL=https://$DOMAINE|" .env
grep '^PUBLIC_BASE_URL=' .env
make restart >/dev/null
echo "  ✓ backend redémarré"

etape "5. Vérification depuis l'extérieur"
code=$(curl -s -o /dev/null -w '%{http_code}' "https://$DOMAINE/api/health")
if [ "$code" != "200" ]; then
  echo "  ✗ https://$DOMAINE/api/health répond $code — port 9999 laissé ouvert par sécurité."
  exit 1
fi
echo "  ✓ https://$DOMAINE/api/health répond 200"

etape "6. Fermeture du port 9999 au public"
sudo ufw delete allow 9999/tcp >/dev/null 2>&1 || true
sudo ufw delete allow 9999 >/dev/null 2>&1 || true
echo "  ✓ le tableau de bord n'est plus joignable qu'en HTTPS"

echo
echo "═══════════════════════════════════════════════════"
echo " Ibalya est en ligne : https://$DOMAINE"
echo
echo " Dernière étape, dans la console Google Cloud :"
echo "   URI de redirection autorisé →"
echo "   https://$DOMAINE/api/oauth/google/callback"
echo " puis Réglages → Connecter Gmail (sans tunnel SSH)."
echo "═══════════════════════════════════════════════════"
