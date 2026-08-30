#!/usr/bin/env python3
"""Génère fixtures/corpus_eval.json : un corpus étiqueté pour mesurer l'extraction.

Le scénario « Menuiserie Dupont » de gen_fixtures.py sert à déclencher les cinq
détecteurs pour une démonstration. Il ne dit pas ce qu'on attend de chaque
message, donc il ne mesure rien.

Ce corpus-ci porte une étiquette par message : y a-t-il un engagement, et
lequel. Il permet de dire si une modification de l'extraction améliore ou
dégrade, ce qui était jusqu'ici impossible.

Il contient délibérément les cas que le scénario de démonstration ignore :
l'accord bref qui engage, la politesse qui n'engage rien, l'échéance relative,
le mail transféré, la citation d'une vieille promesse déjà tenue, la question
qui ressemble à une promesse. Ce sont eux qui séparent une extraction correcte
d'une extraction chanceuse.

AUCUN de ces messages n'est réel : ils sont écrits pour ce corpus. Rien de la
messagerie d'un client ne doit entrer ici.
"""
import json
from datetime import date, timedelta
from pathlib import Path

TODAY = date.today()


def j(n: int) -> str:
    """Date à n jours d'aujourd'hui, au format ISO."""
    return (TODAY + timedelta(days=n)).isoformat()


def fr(n: int) -> str:
    """La même, au format français qu'écrivent les gens."""
    return (TODAY + timedelta(days=n)).strftime("%d/%m/%Y")


DIRIGEANT = "dirigeant@menuiserie-dupont.fr"

# Chaque entrée : le message, puis ce qu'on attend de l'extraction.
#   engagement_attendu : True s'il doit en sortir au moins un
#   echeance_attendue  : la date, ou None si aucune n'est déterminable
#   filtre_attendu     : le message doit-il être écarté AVANT le modèle
#   pourquoi           : ce que ce cas met à l'épreuve
#
# Les deux étages se mesurent séparément. Le pré-filtre décide qui atteint le
# modèle, pas ce qu'il contient : lui reprocher de ne pas reconnaître une
# formule de politesse n'aurait aucun sens.
CAS = [
    # --- engagements francs -------------------------------------------------
    {
        "sujet": "Commande 12 vitrages doubles",
        "de": "commandes@vitrages-pro.fr",
        "corps": f"Bonjour, nous vous confirmons la commande. Nous livrerons "
                 f"l'atelier au plus tard le {fr(6)}. Cordialement, Sophie Marin",
        "engagement_attendu": True, "echeance_attendue": j(6),
        "pourquoi": "promesse explicite, échéance en toutes lettres",
    },
    {
        "sujet": "Chantier école — pose des fenêtres",
        "de": DIRIGEANT,
        "corps": f"Bonjour, je vous confirme la pose des 12 fenêtres de l'école "
                 f"le {fr(9)}. L'équipe sera sur place dès 8h.",
        "engagement_attendu": True, "echeance_attendue": j(9),
        "pourquoi": "engagement du dirigeant lui-même, sens sortant",
    },
    # --- le piège des accords brefs ----------------------------------------
    {
        "sujet": "RE: Devis pergola",
        "de": "m.leroy@particulier.fr",
        "corps": "Ok pour le devis, je vous fais le virement demain.",
        # L'échéance n'est volontairement pas vérifiée ici. Le modèle date
        # « demain » tantôt depuis l'envoi du message, tantôt depuis le jour de
        # l'analyse, et les deux se défendent. Ce que ce cas doit prouver, c'est
        # que trois mots suffisent à engager — pas que la date tombe juste.
        # L'ambiguïté elle-même est traitée ailleurs : une échéance déduite est
        # marquée comme telle et attend confirmation du dirigeant.
        "engagement_attendu": True, "echeance_attendue": None,
        "pourquoi": "trois mots qui engagent ; échéance relative, donc non vérifiée",
    },
    {
        "sujet": "RE: Planning de la semaine",
        "de": "atelier@menuiserie-dupont.fr",
        "corps": "Bien reçu, merci.",
        "engagement_attendu": False, "echeance_attendue": None,
        "pourquoi": "accusé de réception : ressemble à un accord, n'engage rien",
    },
    # --- politesse sans engagement -----------------------------------------
    {
        "sujet": "Suite à notre échange",
        "de": "contact@archi-plus.fr",
        "corps": "Bonjour, merci pour votre temps ce matin. Nous restons à votre "
                 "disposition pour toute question. Bien cordialement.",
        "engagement_attendu": False, "echeance_attendue": None,
        "pourquoi": "formule de courtoisie, aucune action promise",
    },
    {
        "sujet": "Idées pour l'atelier",
        "de": "atelier@menuiserie-dupont.fr",
        "corps": "Il faudrait qu'on pense à réorganiser le stock un de ces jours, "
                 "ça devient compliqué de circuler.",
        "engagement_attendu": False, "echeance_attendue": None,
        "pourquoi": "intention vague sans acteur ni date",
    },
    # --- échéances relatives ------------------------------------------------
    {
        "sujet": "Vernis teinte noyer",
        "de": "commandes@quincaillerie-sud.fr",
        "corps": "Bonjour, nous vous expédions les bidons d'ici la fin de la semaine.",
        "engagement_attendu": True, "echeance_attendue": None,
        "pourquoi": "promesse réelle, échéance floue : ne doit pas être inventée",
    },
    # --- le mail transféré --------------------------------------------------
    {
        "sujet": "Fwd: Confirmation intervention",
        "de": "assistante@menuiserie-dupont.fr",
        "corps": "Pour info.\n\n---------- Message transféré ----------\n"
                 f"De : sav@fournisseur-x.fr\nNous interviendrons le {fr(4)} "
                 "pour le remplacement du moteur.",
        "engagement_attendu": True, "echeance_attendue": j(4),
        "pourquoi": "l'engagement est dans la partie transférée, pas dans l'entête",
    },
    # --- la citation d'une promesse déjà tenue ------------------------------
    {
        "sujet": "RE: RE: Livraison vitrages",
        "de": "commandes@vitrages-pro.fr",
        "corps": "C'est bien arrivé hier, merci de votre retour.\n\n"
                 f"> Le {fr(-8)}, vitrages-pro a écrit :\n"
                 f"> Nous livrerons l'atelier au plus tard le {fr(-6)}.",
        "engagement_attendu": False, "echeance_attendue": None,
        "pourquoi": "la promesse citée est tenue : ne pas la ré-extraire",
    },
    # --- la question qui ressemble à une promesse ---------------------------
    {
        "sujet": "Disponibilité pose",
        "de": "services-techniques@mairie-valbonne.fr",
        "corps": f"Bonjour, pourriez-vous poser les menuiseries avant le {fr(12)} ? "
                 "Merci de me confirmer.",
        "engagement_attendu": True, "echeance_attendue": j(12),
        "pourquoi": "demande avec échéance : engage le destinataire, pas l'émetteur",
    },
    # --- le message automatique qui doit être filtré avant le modèle --------
    {
        "sujet": "Votre facture est disponible",
        "de": "no-reply@facturation-saas.com",
        "corps": f"Votre facture du mois est disponible. Prélèvement le {fr(3)}.",
        "engagement_attendu": False, "echeance_attendue": None,
        "filtre_attendu": True,
        "pourquoi": "expéditeur automatique : doit être écarté par le pré-filtre",
    },
    # --- la catégorie sensible ---------------------------------------------
    {
        "sujet": "Candidature poste menuisier",
        "de": "j.moreau@email.fr",
        "corps": "Bonjour, je vous transmets ma lettre de motivation et mon CV. "
                 f"Je reste disponible pour un entretien à partir du {fr(5)}.",
        "engagement_attendu": False, "echeance_attendue": None,
        "filtre_attendu": True,
        "pourquoi": "catégorie RH : écartée avant le modèle malgré une vraie promesse",
    },
]


def main() -> int:
    corpus = []
    for i, c in enumerate(CAS, start=1):
        corpus.append({
            "external_id": f"eval-{i:03d}",
            "thread_id": f"fil-eval-{i:03d}",
            "subject": c["sujet"],
            "sender": c["de"],
            "sender_name": "",
            "recipients": [DIRIGEANT] if c["de"] != DIRIGEANT else ["client@exemple.fr"],
            "sent_at": "-1d",
            "body": c["corps"],
            "attendu": {
                "engagement": c["engagement_attendu"],
                "echeance": c["echeance_attendue"],
                "filtre": c.get("filtre_attendu", False),
                "pourquoi": c["pourquoi"],
            },
        })
    sortie = Path(__file__).resolve().parent.parent / "fixtures" / "corpus_eval.json"
    sortie.write_text(json.dumps(corpus, ensure_ascii=False, indent=2) + "\n")
    positifs = sum(1 for c in corpus if c["attendu"]["engagement"])
    filtres = sum(1 for c in corpus if c["attendu"]["filtre"])
    print(f"  {sortie.name} : {len(corpus)} cas — {filtres} à écarter par le "
          f"pré-filtre, {positifs} avec engagement attendu, "
          f"{len(corpus) - positifs - filtres} sans engagement")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
