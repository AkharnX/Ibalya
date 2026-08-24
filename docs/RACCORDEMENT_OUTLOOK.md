# Raccorder Outlook — enregistrement Azure

Le connecteur Outlook est prêt côté application, mais il ne peut rien faire
sans une application enregistrée dans Azure. Cette étape n'est pas
automatisable : elle demande un compte Microsoft et quelques minutes dans le
portail. C'est la même démarche que pour Gmail avec la console Google.

## Pourquoi OAuth et pas IMAP

Microsoft a désactivé l'authentification simple sur Exchange Online, et
annoncé sa fin pour les comptes personnels. Le connecteur IMAP d'Ibalya couvre
Yahoo, OVH, Gandi, Orange, Free et l'auto-hébergé — pas Outlook. Seul OAuth 2.0
donne accès à ces boîtes.

## Les étapes

1. Aller sur **portal.azure.com**, section *Microsoft Entra ID* →
   *Inscriptions d'applications* → *Nouvelle inscription*.

2. **Nom** : Ibalya. **Types de comptes pris en charge** : « Comptes dans
   n'importe quel annuaire organisationnel et comptes Microsoft personnels »,
   sauf si vous ne visez qu'une seule organisation.

3. **URI de redirection**, type *Web* :

   ```
   https://ibalya.com/api/oauth/microsoft/callback
   ```

   L'adresse doit correspondre exactement, y compris le protocole. Une adresse
   de développement s'ajoute comme seconde URI.

4. Noter l'**ID d'application (client)** affiché sur la page de vue d'ensemble.

5. *Certificats et secrets* → *Nouveau secret client*. Noter la **valeur**
   immédiatement : elle ne sera plus affichée ensuite. Noter aussi sa date
   d'expiration — un secret expiré coupe le raccordement sans prévenir.

6. *Autorisations d'API* → *Ajouter* → *Microsoft Graph* → *Autorisations
   déléguées*, et cocher :

   | Autorisation | À quoi elle sert |
   |---|---|
   | `offline_access` | Sans elle, aucun jeton de rafraîchissement : le raccordement expire au bout d'une heure |
   | `User.Read` | Lire l'adresse du compte raccordé |
   | `Mail.Read` | Lire les messages |
   | `Mail.Send` | Envoyer les messages que le dirigeant a validés |

   Sur un compte d'entreprise, un administrateur peut devoir accorder le
   consentement pour l'organisation.

## Côté serveur

Ajouter au fichier `.env`, puis redémarrer le service :

```
MICROSOFT_CLIENT_ID=...
MICROSOFT_CLIENT_SECRET=...
# « common » accepte professionnels et personnels ; un identifiant de
# locataire restreint à une seule organisation.
MICROSOFT_TENANT=common
```

## Ensuite

Dans **Réglages › Connexion**, choisir *Outlook* et cliquer sur *Connecter
Outlook*. Le parcours se déroule chez Microsoft ; Ibalya ne reçoit qu'un jeton
révocable, jamais le mot de passe.

## Ce que Graph apporte de plus qu'IMAP

Le fil de discussion est fourni nativement par `conversationId`, là où IMAP
oblige à le reconstruire depuis les en-têtes. Et chaque message porte un lien
profond vers l'interface Outlook, comme le fait Gmail.
