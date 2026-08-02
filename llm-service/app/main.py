"""Service LLM d'AgentOS PME — API interne (jamais exposée publiquement).

Porte l'extraction, la génération de capsule et la rédaction de brouillons
(règle de séparation des responsabilités, CDC section 4).
"""
import json
import logging
from datetime import date

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field

from .prompts import CAPSULE_SYSTEM, DRAFT_SYSTEM, EXTRACTION_SYSTEM
from .provider import get_provider

logging.basicConfig(level=logging.INFO)
log = logging.getLogger("llm-service")

app = FastAPI(title="AgentOps LLM Service", docs_url=None, redoc_url=None)
provider = get_provider()


class MessageIn(BaseModel):
    id: int
    sender: str = ""
    to: str = ""
    subject: str = ""
    body: str = ""
    sent_at: str = ""


class OpenEngagement(BaseModel):
    id: int
    objet: str


class ExtractRequest(BaseModel):
    messages: list[MessageIn]
    open_engagements: list[OpenEngagement] | None = None
    # Any : une capsule difforme ne doit JAMAIS faire échouer l'extraction
    capsule: object = None
    account_email: str = ""

    @property
    def engagements_ouverts(self) -> list[OpenEngagement]:
        return self.open_engagements or []


class ExtractedEngagement(BaseModel):
    emetteur_email: str = ""
    destinataire_email: str = ""
    objet: str = ""
    echeance: str = ""
    echeance_inferee: bool = False
    confiance: float = 0.0


class EngagementUpdate(BaseModel):
    engagement_id: int
    type: str


class ExtractResult(BaseModel):
    message_id: int
    engagements: list[ExtractedEngagement] = Field(default_factory=list)
    updates: list[EngagementUpdate] = Field(default_factory=list)


class ExtractResponse(BaseModel):
    results: list[ExtractResult]


@app.get("/health")
async def health():
    return {"status": "ok"}


@app.post("/extract", response_model=ExtractResponse)
async def extract(req: ExtractRequest):
    payload = {
        "date_du_jour": date.today().isoformat(),
        "compte_du_dirigeant": req.account_email,
        "capsule_client": req.capsule if isinstance(req.capsule, dict) else {},
        "engagements_ouverts": [e.model_dump() for e in req.engagements_ouverts],
        "messages": [m.model_dump() for m in req.messages],
    }
    try:
        raw = await provider.complete_json(EXTRACTION_SYSTEM, json.dumps(payload, ensure_ascii=False))
    except Exception as exc:  # noqa: BLE001
        log.error("extraction: %s", exc)
        raise HTTPException(status_code=502, detail=f"fournisseur LLM: {exc}") from exc

    results: list[ExtractResult] = []
    known_ids = {e.id for e in req.engagements_ouverts}
    for r in raw.get("results", []):
        try:
            res = ExtractResult.model_validate(r)
        except Exception:  # noqa: BLE001
            continue
        # garde-fou : ne jamais accepter une mise à jour vers un engagement inconnu
        res.updates = [u for u in res.updates if u.engagement_id in known_ids]
        # garde-fou : clamp de la confiance
        for e in res.engagements:
            e.confiance = max(0.0, min(1.0, e.confiance))
        results.append(res)
    # tout message sans résultat reçoit une entrée vide (le backend le marque analysé)
    seen = {r.message_id for r in results}
    for m in req.messages:
        if m.id not in seen:
            results.append(ExtractResult(message_id=m.id))
    return ExtractResponse(results=results)


class CapsuleRequest(BaseModel):
    messages_sample: list[MessageIn]
    account_email: str = ""


@app.post("/capsule")
async def capsule(req: CapsuleRequest):
    payload = {
        "compte_du_dirigeant": req.account_email,
        "echantillon": [m.model_dump() for m in req.messages_sample],
    }
    try:
        raw = await provider.complete_json(CAPSULE_SYSTEM, json.dumps(payload, ensure_ascii=False))
    except Exception as exc:  # noqa: BLE001
        log.error("capsule: %s", exc)
        raise HTTPException(status_code=502, detail=f"fournisseur LLM: {exc}") from exc
    facts = raw.get("facts", raw)
    if not isinstance(facts, dict):
        raise HTTPException(status_code=502, detail="capsule non conforme (facts doit être un objet)")
    return {"facts": facts}


class DraftRequest(BaseModel):
    detection_type: str = ""
    detection_titre: str = ""
    detection_detail: str = ""
    engagement_objet: str = ""
    to_email: str = ""
    to_name: str = ""
    from_email: str = ""
    capsule: dict | None = None


@app.post("/draft")
async def draft(req: DraftRequest):
    try:
        raw = await provider.complete_json(DRAFT_SYSTEM, json.dumps(req.model_dump(), ensure_ascii=False))
    except Exception as exc:  # noqa: BLE001
        log.error("draft: %s", exc)
        raise HTTPException(status_code=502, detail=f"fournisseur LLM: {exc}") from exc
    subject = str(raw.get("subject", "")).strip() or "Suivi"
    body = str(raw.get("body", "")).strip()
    if not body:
        raise HTTPException(status_code=502, detail="brouillon vide")
    return {"subject": subject, "body": body}
