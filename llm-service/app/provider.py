"""Abstraction du fournisseur LLM (exigence de réversibilité, CDC section 13).

Le reste du service ne connaît que `LLMProvider.complete_json()` : changer de
fournisseur = ajouter une classe, sans refonte.
"""
import json
import os
from abc import ABC, abstractmethod

import httpx


class LLMProvider(ABC):
    @abstractmethod
    async def complete_json(self, system: str, user: str) -> dict:
        """Retourne la réponse du modèle parsée en JSON."""


class MistralProvider(LLMProvider):
    BASE_URL = "https://api.mistral.ai/v1/chat/completions"

    def __init__(self) -> None:
        self.api_key = os.environ.get("MISTRAL_API_KEY", "")
        self.model = os.environ.get("MISTRAL_MODEL", "mistral-large-latest")
        if not self.api_key:
            raise RuntimeError("MISTRAL_API_KEY manquant")

    async def complete_json(self, system: str, user: str) -> dict:
        async with httpx.AsyncClient(timeout=90) as client:
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
            resp.raise_for_status()
            content = resp.json()["choices"][0]["message"]["content"]
            return json.loads(content)


class MockProvider(LLMProvider):
    """Fournisseur factice pour tester la chaîne sans clé d'API.

    Extraction naïve par mots-clés — uniquement pour la validation technique,
    jamais en production.
    """

    async def complete_json(self, system: str, user: str) -> dict:
        if "extraction d'engagements" in system:
            try:
                payload = json.loads(user)
                results = []
                keywords = ("livr", "envoi", "enverr", "promis", "d'ici", "avant le",
                            "je vous", "je te", "on vous", "sera prêt", "deadline")
                for m in payload.get("messages", []):
                    body = (m.get("body") or "").lower()
                    engs = []
                    if any(k in body for k in keywords):
                        engs.append({
                            "emetteur_email": m.get("sender", ""),
                            "destinataire_email": (m.get("to") or "").split(",")[0].strip(),
                            "objet": (m.get("subject") or "Engagement détecté")[:150],
                            "echeance": "",
                            "echeance_inferee": False,
                            "confiance": 0.65,
                        })
                    results.append({"message_id": m.get("id"), "engagements": engs, "updates": []})
                return {"results": results}
            except Exception:
                return {"results": []}
        if "capsule" in system:
            return {"facts": {"secteur": "inconnu (mock)", "horizon_jours": 7}}
        return {"subject": "Suivi", "body": "Bonjour,\n\nOù en sommes-nous ?\n\nCordialement"}


def get_provider() -> LLMProvider:
    name = os.environ.get("LLM_PROVIDER", "mistral")
    if name == "mock":
        return MockProvider()
    return MistralProvider()
