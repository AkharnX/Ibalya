package ingest

import (
	"testing"

	"ibalya/backend/internal/store"
)

// La classe de l'expéditeur décide ce qui part à l'extraction. Un automatique
// pris pour un humain fabrique un faux engagement « en retard » ; un humain pris
// pour un automatique fait rater une promesse (constat BS ACCESS, 11 nov. 2026).
func TestClasserExpediteur(t *testing.T) {
	cas := []struct {
		nom    string
		m      store.Message
		classe Classe
		motif  string
	}{
		{"suivi de colis d'une place de marché", store.Message{Sender: "shipment-tracking@amazon.fr", Body: "Votre colis est en route"}, ClasseAutomatique, "domaine_exclu"},
		{"notification d'opérateur", store.Message{Sender: "no-reply@notification.orange.fr", Body: "Votre facture est disponible"}, ClasseAutomatique, "expediteur_automatique"},
		{"commercial d'opérateur", store.Message{Sender: "marc.dupont@orange.com", Body: "Bonjour, je vous confirme la pose des bornes le 12/11. Marc"}, ClasseHumain, ""},
		{"corps qui se déclare automatique", store.Message{Sender: "contact@logiciel-gestion.fr", Body: "Ceci est un message automatique, merci de ne pas y répondre."}, ClasseAutomatique, "expediteur_automatique"},
		{"expédition annoncée par une personne", store.Message{Sender: "julie@fournisseur.fr", Body: "Bonjour, votre commande a été expédiée ce matin. Cordialement, Julie"}, ClasseHumain, ""},
		{"adresse commandes@ d'un vrai fournisseur", store.Message{Sender: "commandes@vitrages-pro.fr", Body: "Bonjour, nous vous confirmons la commande. Nous livrerons au plus tard le 03/11."}, ClasseHumain, ""},
		{"en-tête d'envoi automatique", store.Message{Sender: "support@outil.fr", Body: "Votre ticket a été mis à jour", AutoSubmitted: true}, ClasseAutomatique, "expediteur_automatique"},
		{"désinscription seule", store.Message{Sender: "contact@boutique.fr", Body: "Nos promotions", ListUnsubscribe: true}, ClassePublicitaire, "newsletter"},
		{"objet hameçonnage", store.Message{Sender: "securite@banque-alerte.com", Subject: "Connexion suspecte sur votre compte", Body: "Cliquez ici"}, ClasseSecurite, "securite_suspect"},
		{"urgence artificielle", store.Message{Sender: "service@compte-client.net", Body: "Votre compte sera suspendu sous 24h, vérifiez votre identité."}, ClasseSecurite, "securite_suspect"},
		{"demande humaine de coordonnées : un seul signal faible", store.Message{Sender: "sophie@client.fr", Body: "Pouvez-vous confirmer vos coordonnées pour la livraison ?"}, ClasseHumain, ""},
		{"message ordinaire", store.Message{Sender: "client@exemple.fr", Body: "Bonjour, où en est le devis ?"}, ClasseHumain, ""},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			classe, motif := ClasserExpediteur(c.m)
			if classe != c.classe || motif != c.motif {
				t.Fatalf("attendu %s/%q, obtenu %s/%q", c.classe, c.motif, classe, motif)
			}
		})
	}
}
