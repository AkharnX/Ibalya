"""Garde-fous du service d'inférence.

Le modèle peut renvoyer n'importe quoi : ces tests vérifient que rien
d'aberrant ne franchit la frontière entre le LLM et le backend.
"""
import os
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
os.environ.setdefault("LLM_PROVIDER", "mock")

from fastapi.testclient import TestClient  # noqa: E402
from app.main import app  # noqa: E402

client = TestClient(app)


def test_sante():
    assert client.get("/health").json() == {"status": "ok"}


def test_extraction_repond_pour_chaque_message():
    """Un message sans engagement doit tout de même recevoir une entrée :
    sinon le backend ne le marquerait jamais comme analysé et le relirait
    indéfiniment."""
    r = client.post("/extract", json={
        "messages": [
            {"id": 1, "sender": "a@x.fr", "body": "Je vous livre le 05/09/2026."},
            {"id": 2, "sender": "b@x.fr", "body": "Merci, bonne journée."},
        ],
        "open_engagements": [], "capsule": {}, "account_email": "moi@x.fr",
    })
    assert r.status_code == 200
    ids = {res["message_id"] for res in r.json()["results"]}
    assert ids == {1, 2}


def test_extraction_rejette_une_maj_vers_un_engagement_inconnu():
    """Une mise à jour vers un identifiant non fourni est écartée : le modèle
    ne doit pas pouvoir modifier un engagement au hasard."""
    from app.main import ExtractRequest, ExtractResult, EngagementUpdate
    req = ExtractRequest(messages=[{"id": 1}], open_engagements=[{"id": 7, "objet": "x"}])
    res = ExtractResult(message_id=1, updates=[
        EngagementUpdate(engagement_id=7, type="livre"),
        EngagementUpdate(engagement_id=999, type="livre"),
    ])
    connus = {e.id for e in req.engagements_ouverts}
    gardees = [u for u in res.updates if u.engagement_id in connus]
    assert [u.engagement_id for u in gardees] == [7]


def test_capsule_difforme_ne_casse_pas_l_extraction():
    """Une capsule qui n'est pas un objet ne doit jamais faire échouer
    l'extraction : elle est simplement ignorée."""
    for capsule in (None, "texte", 42, ["a"]):
        r = client.post("/extract", json={
            "messages": [{"id": 1, "sender": "a@x.fr", "body": "Bonjour"}],
            "capsule": capsule,
        })
        assert r.status_code == 200, f"capsule {capsule!r} a fait échouer l'extraction"


def test_relecture_structure_la_reponse():
    r = client.post("/review", json={
        "to_email": "client@x.fr", "subject": "Suivi",
        "body": "Bonjour, merci.",
    })
    assert r.status_code == 200
    d = r.json()
    assert d["verdict"] in ("pret_a_envoyer", "a_revoir")
    assert isinstance(d["remarques"], list)
    for rem in d["remarques"]:
        assert rem["type"] in {"factuel", "manque", "risque", "ton"}
        assert rem["message"]


def test_relecture_refuse_un_message_vide():
    r = client.post("/review", json={"to_email": "c@x.fr", "body": ""})
    assert r.status_code in (200, 422)
    if r.status_code == 200:
        assert r.json()["verdict"] == "a_revoir"


# --- reprise sur limitation de débit -----------------------------------------

def test_reprise_sur_429(monkeypatch):
    """Un 429 doit être réessayé, pas propagé au premier coup.

    Sans reprise, une rafale d'appels interrompait le cycle d'extraction : rien
    n'était perdu, mais l'analyse restait bloquée jusqu'au cycle suivant.
    """
    import asyncio
    import httpx
    from app import provider as mod

    monkeypatch.setenv("MISTRAL_API_KEY", "cle-de-test")
    p = mod.MistralProvider()
    p.ATTENTE_INITIALE = 0.0  # pas d'attente réelle en test

    appels = {"n": 0}

    class FausseReponse:
        def __init__(self, code, corps=None):
            self.status_code = code
            self.headers = {}
            self._corps = corps

        def json(self):
            return self._corps

        def raise_for_status(self):
            if self.status_code >= 400:
                raise httpx.HTTPStatusError(
                    f"statut {self.status_code}", request=None, response=None)

    class FauxClient:
        async def __aenter__(self):
            return self

        async def __aexit__(self, *a):
            return False

        async def post(self, *a, **k):
            appels["n"] += 1
            if appels["n"] < 3:
                return FausseReponse(429)
            return FausseReponse(200, {
                "choices": [{"message": {"content": '{"engagements": []}'}}]})

    monkeypatch.setattr(mod.httpx, "AsyncClient", lambda **k: FauxClient())
    out = asyncio.run(p.complete_json("sys", "usr"))
    assert out == {"engagements": []}
    assert appels["n"] == 3, "les deux premiers 429 auraient dû être réessayés"


def test_abandon_apres_trop_de_429(monkeypatch):
    """La reprise n'est pas infinie : au bout du compte, l'erreur remonte."""
    import asyncio
    import httpx
    import pytest
    from app import provider as mod

    monkeypatch.setenv("MISTRAL_API_KEY", "cle-de-test")
    p = mod.MistralProvider()
    p.ATTENTE_INITIALE = 0.0

    class FausseReponse:
        status_code = 429
        headers: dict = {}

        def raise_for_status(self):
            raise httpx.HTTPStatusError("429", request=None, response=None)

    class FauxClient:
        async def __aenter__(self):
            return self

        async def __aexit__(self, *a):
            return False

        async def post(self, *a, **k):
            return FausseReponse()

    monkeypatch.setattr(mod.httpx, "AsyncClient", lambda **k: FauxClient())
    with pytest.raises(httpx.HTTPStatusError):
        asyncio.run(p.complete_json("sys", "usr"))
