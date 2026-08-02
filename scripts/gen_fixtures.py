#!/usr/bin/env python3
"""Génère fixtures/messages.json avec des dates relatives au jour d'exécution.

Scénario « Menuiserie Dupont » conçu pour déclencher les 5 détecteurs :
  1. Échéance à risque   : pose fenêtres mairie dans 4 jours, aucun signal
  2. Silence anormal     : fournisseur vitrage muet depuis 6 jours
  3. Contradiction       : la pose (aval) dépend du vitrage (amont) en retard
  4. Orphelin            : devis pergola promis il y a 12 jours, aucune suite
  5. Surcharge           : 5 échéances la même semaine pour le dirigeant
"""
import json
import sys
from datetime import date, timedelta
from pathlib import Path

TODAY = date.today()
FR = "%d/%m/%Y"


def days(n: int) -> str:
    return f"-{n}d" if n >= 0 else f"{-n}d"


def fr(d: date) -> str:
    return d.strftime(FR)


# prochaine semaine complète (lundi → vendredi) pour la surcharge
next_monday = TODAY + timedelta(days=(7 - TODAY.weekday()) % 7 or 7)

vitrage_due = TODAY - timedelta(days=3)      # promesse fournisseur dépassée
pose_due = TODAY + timedelta(days=4)         # promesse au client, imminente
DIRIGEANT = "dirigeant@menuiserie-dupont.fr"

messages = [
    # --- fil mairie : engagement aval (pose) ---
    {
        "external_id": "fx-101", "thread_id": "th-mairie",
        "subject": "Chantier école — pose des fenêtres",
        "sender": DIRIGEANT, "sender_name": "Marc Dupont",
        "recipients": ["services-techniques@mairie-valbonne.fr"],
        "sent_at": days(15),
        "body": f"Bonjour, je vous confirme la pose des 12 fenêtres de l'école le {fr(pose_due)}. L'équipe sera sur place dès 8h. Cordialement, Marc Dupont",
    },
    {
        "external_id": "fx-102", "thread_id": "th-mairie",
        "subject": "RE: Chantier école — pose des fenêtres",
        "sender": "services-techniques@mairie-valbonne.fr", "sender_name": "Mairie de Valbonne",
        "recipients": [DIRIGEANT],
        "sent_at": days(14),
        "body": "Bonjour M. Dupont, parfait, c'est noté. La commission de sécurité passe la semaine suivante, le calendrier est serré. Bien à vous.",
    },
    # --- fil vitrage : engagement amont en retard + silence ---
    {
        "external_id": "fx-103", "thread_id": "th-vitrage",
        "subject": "Commande 12 vitrages doubles — confirmation",
        "sender": "commandes@vitrages-pro.fr", "sender_name": "Vitrages Pro",
        "recipients": [DIRIGEANT],
        "sent_at": days(13),
        "body": f"Bonjour, nous vous confirmons la commande. Nous livrerons l'atelier au plus tard le {fr(vitrage_due)}. Cordialement, le service commandes.",
    },
    {
        "external_id": "fx-104", "thread_id": "th-vitrage",
        "subject": "RE: Commande 12 vitrages doubles — confirmation",
        "sender": DIRIGEANT, "sender_name": "Marc Dupont",
        "recipients": ["commandes@vitrages-pro.fr"],
        "sent_at": days(6),
        "body": "Bonjour, où en est la livraison ? Nous approchons de la date et je n'ai pas de nouvelles du transporteur. Merci de me tenir informé. Marc Dupont",
    },
    # --- devis pergola : orphelin (promis il y a 12 jours, aucune suite) ---
    {
        "external_id": "fx-105", "thread_id": "th-pergola",
        "subject": "Devis pergola bioclimatique",
        "sender": "paul.rossi@gmail.com", "sender_name": "Paul Rossi",
        "recipients": [DIRIGEANT],
        "sent_at": days(13),
        "body": "Bonjour, merci pour votre visite. Pouvez-vous m'envoyer le devis pour la pergola ? Nous aimerions démarrer rapidement.",
    },
    {
        "external_id": "fx-106", "thread_id": "th-pergola",
        "subject": "RE: Devis pergola bioclimatique",
        "sender": DIRIGEANT, "sender_name": "Marc Dupont",
        "recipients": ["paul.rossi@gmail.com"],
        "sent_at": days(12),
        "body": "Bonjour M. Rossi, bien reçu. Je vous prépare le devis complet et je vous l'envoie d'ici la fin de semaine. Bien cordialement, Marc",
    },
    # --- SAV : engagement sans échéance explicite (inférée) ---
    {
        "external_id": "fx-107", "thread_id": "th-sav",
        "subject": "Porte d'entrée qui frotte",
        "sender": "famille.menard@orange.fr", "sender_name": "Famille Ménard",
        "recipients": [DIRIGEANT],
        "sent_at": days(8),
        "body": "Bonjour, la porte posée en mai frotte au sol. Pourriez-vous passer la régler ? Merci d'avance.",
    },
    {
        "external_id": "fx-108", "thread_id": "th-sav",
        "subject": "RE: Porte d'entrée qui frotte",
        "sender": DIRIGEANT, "sender_name": "Marc Dupont",
        "recipients": ["famille.menard@orange.fr"],
        "sent_at": days(7),
        "body": "Bonjour, c'est sous garantie, pas d'inquiétude. Je passe régler la porte la semaine prochaine, je vous appelle avant. Cordialement, Marc",
    },
    # --- prospect : prise de contact + rendez-vous proposé ---
    {
        "external_id": "fx-120", "thread_id": "th-prospect",
        "subject": "Remplacement fenêtres maison années 70",
        "sender": "sophie.laurent@hotmail.fr", "sender_name": "Sophie Laurent",
        "recipients": [DIRIGEANT],
        "sent_at": days(3),
        "body": "Bonjour, nous cherchons un menuisier pour remplacer 8 fenêtres dans notre maison. Seriez-vous disponible pour venir voir le chantier ?",
    },
    {
        "external_id": "fx-121", "thread_id": "th-prospect",
        "subject": "RE: Remplacement fenêtres maison années 70",
        "sender": DIRIGEANT, "sender_name": "Marc Dupont",
        "recipients": ["sophie.laurent@hotmail.fr"],
        "sent_at": days(2),
        "body": f"Bonjour Mme Laurent, avec plaisir. Je vous propose un rendez-vous sur place le {fr(TODAY + timedelta(days=6))} à 10h pour prendre les mesures. Bien cordialement, Marc Dupont",
    },
    # --- newsletter : doit être exclue par le pré-filtre (EF-11) ---
    {
        "external_id": "fx-109", "thread_id": "th-news",
        "subject": "Promotions de l'été — quincaillerie",
        "sender": "no-reply@quincaillerie-plus.fr", "sender_name": "Quincaillerie Plus",
        "recipients": [DIRIGEANT],
        "sent_at": days(5), "list_unsubscribe": True,
        "body": "Découvrez nos promotions sur les charnières et poignées !",
    },
]

# --- surcharge : 5 promesses du dirigeant, échéances la même semaine ---
surcharge = [
    ("th-s1", "volets M. Bianchi", "pose des volets roulants"),
    ("th-s2", "escalier Mme Costa", "vernissage de l'escalier"),
    ("th-s3", "placards cabinet médical", "installation des placards"),
    ("th-s4", "facture chantier Leroy", "envoi de la facture définitive"),
    ("th-s5", "portail M. Girard", "pose du portail coulissant"),
]
for i, (tid, short, longer) in enumerate(surcharge):
    due = next_monday + timedelta(days=i)
    messages.append({
        "external_id": f"fx-2{i:02d}", "thread_id": tid,
        "subject": f"Planning — {short}",
        "sender": DIRIGEANT, "sender_name": "Marc Dupont",
        "recipients": [f"client{i}@exemple.fr"],
        "sent_at": days(4 - i if 4 - i >= 0 else 0),
        "body": f"Bonjour, je vous confirme l'intervention : {longer} le {fr(due)}. Bien cordialement, Marc Dupont",
    })

out = {"account_email": DIRIGEANT, "messages": messages}
path = Path(sys.argv[1] if len(sys.argv) > 1 else "fixtures/messages.json")
path.write_text(json.dumps(out, ensure_ascii=False, indent=2), encoding="utf-8")
print(f"{path} : {len(messages)} messages, dates relatives au {TODAY.isoformat()}")
