# Environnement de test — beta.ibalya.com

Pile **totalement isolée** de la prod, pour éprouver des fonctionnalités (à
commencer par l'assistant conversationnel) sans jamais toucher aux données
réelles.

## Ce qui diffère de la prod

| Élément            | Prod                    | Test (beta)                     |
|--------------------|-------------------------|---------------------------------|
| Code               | `/home/akharn/agnet-ia` | `/home/akharn/agnet-ia-test` (worktree) |
| Base PostgreSQL    | `ibalya-db` `:5435`     | `ibalya-db-test` `:5436` (volume dédié) |
| Backend Go         | `:9999`                 | `:9998`                         |
| Service LLM        | `:8092`                 | `:8093`                         |
| Unités systemd     | `ibalya-backend` / `ibalya-llm` | `ibalya-backend-test` / `ibalya-llm-test` |
| URL publique       | `ibalya.com`            | `beta.ibalya.com`               |
| `.env`             | prod                    | `/home/akharn/agnet-ia-test/.env` (secrets de test dédiés) |

Rien n'est partagé : base, process, secrets et code sont distincts.

## Monter la pile

```bash
cd /home/akharn/agnet-ia-test

# 1. Base de test
DB_PASSWORD=<voir .env> docker-compose -p ibalya-test -f deploy/beta/docker-compose.yml up -d

# 2. Build
cd frontend && npm ci && npm run build && cd ..
cd backend && go build -o bin/ibalya ./cmd/server && cd ..

# 3. Services (systemd)
sudo cp deploy/beta/ibalya-llm-test.service deploy/beta/ibalya-backend-test.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now ibalya-llm-test ibalya-backend-test

# 4. Compte de test
printf '<mot_de_passe>\n' | ./backend/bin/ibalya -creer-utilisateur <email> -nom "<nom>"
```

## Exposer via beta.ibalya.com (préalables externes)

1. **DNS** : enregistrement `A` `beta.ibalya.com` → IP du serveur.
2. **nginx** :
   ```bash
   sudo cp deploy/beta/nginx-beta.ibalya.com.conf /etc/nginx/sites-available/beta.ibalya.com
   sudo ln -s /etc/nginx/sites-available/beta.ibalya.com /etc/nginx/sites-enabled/
   sudo nginx -t && sudo systemctl reload nginx
   ```
3. **TLS** : `sudo certbot --nginx -d beta.ibalya.com`
4. **OAuth Google** : ajouter `https://beta.ibalya.com/api/oauth/google/callback`
   aux URIs de redirection autorisées du client OAuth.

## Redéployer une mise à jour

```bash
cd /home/akharn/agnet-ia-test
git pull        # ou git checkout <branche>
cd frontend && npm run build && cd ..
cd backend && go build -o bin/ibalya ./cmd/server && cd ..
sudo systemctl restart ibalya-llm-test ibalya-backend-test
```
