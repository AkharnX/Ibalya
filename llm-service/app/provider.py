"""Abstraction du fournisseur LLM (exigence de réversibilité, CDC section 13).

Le reste du service ne connaît que `LLMProvider.complete_json()` : changer de
fournisseur = ajouter une classe, sans refonte.
"""
import asyncio
import json
import logging
import os
import re
from abc import ABC, abstractmethod

import httpx

logger = logging.getLogger("llm-service")


class LLMProvider(ABC):
    @abstractmethod
    async def complete_json(self, system: str, user: str) -> dict:
        """Retourne la réponse du modèle parsée en JSON."""


class MistralProvider(LLMProvider):
    BASE_URL = "https://api.mistral.ai/v1/chat/completions"

    def __init__(self) -> None:
        # .strip() indispensable : un commentaire en fin de ligne dans .env laisse
        # un espace dans la valeur vue par make, ce qui rend l'en-tête HTTP invalide.
        self.api_key = os.environ.get("MISTRAL_API_KEY", "").strip()
        self.model = os.environ.get("MISTRAL_MODEL", "mistral-large-latest").strip()
        if not self.api_key:
            raise RuntimeError("MISTRAL_API_KEY manquant")

    # Reprise sur limitation de débit.
    #
    # Mistral renvoie 429 quand les appels arrivent trop vite. Sans reprise, le
    # cycle d'extraction s'interrompait au premier refus et l'ensemble du lot
    # repartait au cycle suivant : rien n'était perdu, mais l'analyse pouvait
    # rester bloquée plusieurs heures sur une rafale de messages.
    #
    # L'attente double à chaque tentative, et l'en-tête Retry-After prime quand
    # le fournisseur l'envoie : il sait mieux que nous quand réessayer.
    TENTATIVES = 4
    ATTENTE_INITIALE = 2.0

    async def complete_json(self, system: str, user: str) -> dict:
        attente = self.ATTENTE_INITIALE
        derniere: Exception | None = None
        async with httpx.AsyncClient(timeout=90) as client:
            for tentative in range(1, self.TENTATIVES + 1):
                resp = await client.post(
                    self.BASE_URL,
                    headers={"Authorization": f"Bearer {self.api_key}"},
                    json={
                        "model": self.model,
                        "temperature": 0.2,
                        "response_format": {"type": "json_object"},
                        "messages": [
                            {"role": "system", "content": system},
                            {"role": "user", "content": user},
                        ],
                    },
                )
                # 429 : trop d'appels. 5xx : incident passager côté fournisseur.
                if resp.status_code == 429 or resp.status_code >= 500:
                    if tentative == self.TENTATIVES:
                        resp.raise_for_status()
                    pause = attente
                    if entete := resp.headers.get("Retry-After"):
                        try:
                            pause = max(pause, float(entete))
                        except ValueError:
                            pass
                    logger.warning(
                        "fournisseur %s : nouvelle tentative dans %.0fs (%d/%d)",
                        resp.status_code, pause, tentative, self.TENTATIVES,
                    )
                    await asyncio.sleep(pause)
                    attente *= 2
                    continue
                resp.raise_for_status()
                content = resp.json()["choices"][0]["message"]["content"]
                return json.loads(content)
        raise derniere or RuntimeError("fournisseur injoignable")


class MockProvider(LLMProvider):
    """Fournisseur factice pour tester la chaîne sans clé d'API.

    Extraction par règles simples (mots-clés + dates JJ/MM/AAAA) — uniquement
    pour la validation technique et la démo, jamais en production.
    """

    DATE_RE = re.compile(r"\b(\d{2})/(\d{2})/(\d{4})\b")
    KEYWORDS = ("livr", "enverr", "envoie", "poser", "je passe", "promis", "d'ici",
                "avant le", "je vous confirme", "sera prêt", "vous transmets",
                "je m'en occupe", "au plus tard")

    def _extract_message(self, m: dict) -> list[dict]:
        body = m.get("body") or ""
        low = body.lower()
        if not any(k in low for k in self.KEYWORDS):
            return []
        echeance, inferee = "", False
        if (match := self.DATE_RE.search(body)):
            d, mo, y = match.groups()
            echeance = f"{y}-{mo}-{d}"
        elif "?" in body:
            return []  # question sans date = demande, pas une promesse
        elif "demain" in low or "semaine prochaine" in low or "vendredi" in low:
            inferee = True  # date relative : le vrai LLM la résoudrait, le mock la signale
        first_line = next((l.strip() for l in body.splitlines() if len(l.strip()) > 20), "")
        objet = (m.get("subject") or "").removeprefix("RE: ").strip() or first_line[:120]
        if "devis" in low:
            etype = "devis"
        elif "rendez-vous" in low or "rdv" in low or "entretien" in low:
            etype = "rendez_vous"
        elif "facture" in low or "règlement" in low:
            etype = "facturation"
        elif "je vous rappelle" in low or "je reviens vers vous" in low:
            etype = "relance"
        elif "pose" in low or "livr" in low or "install" in low:
            etype = "livraison"
        else:
            etype = "autre"
        return [{
            "emetteur_email": m.get("sender", ""),
            "destinataire_email": (m.get("to") or "").split(",")[0].strip(),
            "objet": objet,
            "type": etype,
            "echeance": echeance,
            "echeance_inferee": inferee if not echeance else False,
            "confiance": 0.85 if echeance else 0.7,
        }]

    async def complete_json(self, system: str, user: str) -> dict:
        if "extraction d'engagements" in system:
            try:
                payload = json.loads(user)
                return {"results": [
                    {"message_id": m.get("id"), "engagements": self._extract_message(m), "updates": []}
                    for m in payload.get("messages", [])
                ]}
            except Exception:
                return {"results": []}
        if "Tu relis un message" in system:
            try:
                req = json.loads(user)
            except Exception:
                req = {}
            body = req.get("body") or ""
            remarques = []
            if "?" not in body:
                remarques.append({"type": "manque",
                                  "message": "Le message ne pose aucune question claire : le destinataire ne saura pas quoi répondre."})
            if len(body) > 900:
                remarques.append({"type": "ton", "message": "Message assez long — un dirigeant lit vite, deux paragraphes suffisent."})
            return {"verdict": "a_revoir" if remarques else "pret_a_envoyer",
                    "remarques": remarques, "suggestion": ""}
        if "capsule" in system:
            return {"facts": {
                "secteur": "artisanat / second œuvre (hypothèse à confirmer)",
                "description": "Ce que je comprends : une entreprise artisanale qui gère des chantiers, des devis et du SAV par email avec clients, fournisseurs et collectivités.",
                "clients_recurrents": ["services-techniques@mairie-valbonne.fr"],
                "fournisseurs_critiques": ["commandes@vitrages-pro.fr"],
                "interlocuteurs_cles": ["paul.rossi@gmail.com"],
                "cycle_type": "chantiers de 2 à 6 semaines",
                "horizon_jours": 7,
                "silence_defaut_heures": 96,
            }}
        # brouillon
        try:
            req = json.loads(user)
        except Exception:
            req = {}
        objet = req.get("engagement_objet") or "notre dossier en cours"
        return {
            "subject": f"Suivi — {objet[:60]}",
            "body": (f"Bonjour,\n\nSauf erreur de ma part, je suis sans nouvelles concernant "
                     f"« {objet} ». Pouvez-vous me faire un point rapide sur l'avancement "
                     f"et me confirmer le délai ?\n\nMerci d'avance,\nBien cordialement"),
        }


def get_provider() -> LLMProvider:
    name = os.environ.get("LLM_PROVIDER", "mistral")
    if name == "mock":
        return MockProvider()
    return MistralProvider()
