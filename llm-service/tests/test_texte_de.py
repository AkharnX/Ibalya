from app.provider import texte_de


def test_chaine_inchangee():
    assert texte_de('{"a": 1}') == '{"a": 1}'


def test_liste_de_parties_texte():
    parts = [{"type": "text", "text": '{"a":'}, {"type": "text", "text": " 1}"}]
    assert texte_de(parts) == '{"a": 1}'


def test_liste_ignore_les_parties_non_texte():
    parts = [{"type": "image_url", "image_url": "x"}, {"type": "text", "text": "ok"}, "!"]
    assert texte_de(parts) == "ok!"


def test_vide():
    assert texte_de(None) == ""
