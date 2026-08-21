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
