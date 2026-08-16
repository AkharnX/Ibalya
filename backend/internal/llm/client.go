// Package llm est le client de l'API interne du service LLM (Python isolé).
// Règle de séparation du CDC section 4 : le backend Go ne parle jamais
// directement au fournisseur de modèle.

// TODO : Rajouter de la memoire afin de faire le fine-tuning du modèle sur les données de l'utilisateur (ex: historique des messages).
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func New(baseURL string) *Client {
	return &Client{BaseURL: baseURL, HTTP: &http.Client{Timeout: 120 * time.Second}}
}

// --- Extraction (couche 2) ---

type ExtractMessage struct {
	ID      int64  `json:"id"`
	Sender  string `json:"sender"`
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
	SentAt  string `json:"sent_at"`
}

type OpenEngagement struct {
	ID    int64  `json:"id"`
	Objet string `json:"objet"`
}

type ExtractRequest struct {
	Messages        []ExtractMessage  `json:"messages"`
	OpenEngagements []OpenEngagement  `json:"open_engagements"`
	Capsule         json.RawMessage   `json:"capsule"`
	AccountEmail    string            `json:"account_email"`
}

type ExtractedEngagement struct {
	EmetteurEmail     string  `json:"emetteur_email"`
	DestinataireEmail string  `json:"destinataire_email"`
	Objet             string  `json:"objet"`
	Type              string  `json:"type"` // devis / livraison / relance / prise_de_contact / rendez_vous / facturation / autre
	Echeance          string  `json:"echeance"` // YYYY-MM-DD ou ""
	EcheanceInferee   bool    `json:"echeance_inferee"`
	Confiance         float64 `json:"confiance"`
}

type EngagementUpdate struct {
	EngagementID int64  `json:"engagement_id"`
	Type         string `json:"type"` // signal_progression / livre / relance / abandonne
}

type ExtractResult struct {
	MessageID   int64                 `json:"message_id"`
	Engagements []ExtractedEngagement `json:"engagements"`
	Updates     []EngagementUpdate    `json:"updates"`
}

type ExtractResponse struct {
	Results []ExtractResult `json:"results"`
}

func (c *Client) Extract(ctx context.Context, req ExtractRequest) (*ExtractResponse, error) {
	var resp ExtractResponse
	if err := c.post(ctx, "/extract", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// --- Capsule (temps 1 : inférence des faits) ---

type CapsuleRequest struct {
	MessagesSample []ExtractMessage `json:"messages_sample"`
	AccountEmail   string           `json:"account_email"`
}

type CapsuleResponse struct {
	Facts json.RawMessage `json:"facts"`
}

func (c *Client) InferCapsule(ctx context.Context, req CapsuleRequest) (*CapsuleResponse, error) {
	var resp CapsuleResponse
	if err := c.post(ctx, "/capsule", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// --- Brouillons d'action (EF-7) ---

type DraftRequest struct {
	DetectionType  string          `json:"detection_type"`
	DetectionTitre string          `json:"detection_titre"`
	DetectionDetail string         `json:"detection_detail"`
	EngagementObjet string         `json:"engagement_objet"`
	ToEmail        string          `json:"to_email"`
	ToName         string          `json:"to_name"`
	FromEmail      string          `json:"from_email"`
	Capsule        json.RawMessage `json:"capsule"`
	// derniers échanges du fil : le brouillon doit coller à la conversation
	ThreadExtraits []string `json:"thread_extraits"`
	// intention explicite (relancer, informer d'un retard, confirmer une date…)
	Intent      string `json:"intent"`
	IntentLabel string `json:"intent_label"`
	// historique partagé avec cet interlocuteur, tous fils confondus
	ContexteClient []string `json:"contexte_client"`
}

// --- Relecture d'un message modifié par le dirigeant ---

type ReviewRequest struct {
	ToEmail        string          `json:"to_email"`
	Subject        string          `json:"subject"`
	Body           string          `json:"body"`
	Intent         string          `json:"intent"`
	IntentLabel    string          `json:"intent_label"`
	EngagementObjet string         `json:"engagement_objet"`
	Contexte       string          `json:"contexte"`
	ContexteClient []string        `json:"contexte_client"`
	ThreadExtraits []string        `json:"thread_extraits"`
	Capsule        json.RawMessage `json:"capsule"`
}

type ReviewRemark struct {
	Type    string `json:"type"` // manque / ton / risque / ok
	Message string `json:"message"`
}

type ReviewResponse struct {
	Verdict    string         `json:"verdict"` // pret_a_envoyer / a_revoir
	Remarques  []ReviewRemark `json:"remarques"`
	Suggestion string         `json:"suggestion"` // version améliorée, facultative
}

func (c *Client) Review(ctx context.Context, req ReviewRequest) (*ReviewResponse, error) {
	var resp ReviewResponse
	if err := c.post(ctx, "/review", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type DraftResponse struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

func (c *Client) Draft(ctx context.Context, req DraftRequest) (*DraftResponse, error) {
	var resp DraftResponse
	if err := c.post(ctx, "/draft", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Health(ctx context.Context) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", c.BaseURL+"/health", nil)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("service LLM: statut %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) post(ctx context.Context, path string, in, out any) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("service LLM injoignable (%s): %w", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("service LLM %s: statut %d: %s", path, resp.StatusCode, truncate(string(body), 300))
	}
	return json.Unmarshal(body, out)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
