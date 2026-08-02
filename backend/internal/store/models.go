package store

import (
	"encoding/json"
	"time"
)

type Person struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Type      string    `json:"type"`
	Sensitive bool      `json:"sensitive"`
	CreatedAt time.Time `json:"created_at"`
}

type Thread struct {
	ID                  int64      `json:"id"`
	Channel             string     `json:"channel"`
	ExternalID          string     `json:"external_id"`
	Subject             string     `json:"subject"`
	LastMessageAt       *time.Time `json:"last_message_at"`
	ResponseRhythmHours *float64   `json:"response_rhythm_hours"`
	Excluded            bool       `json:"excluded"`
}

type Message struct {
	ID              int64     `json:"id"`
	ThreadID        int64     `json:"thread_id"`
	ExternalID      string    `json:"external_id"`
	Channel         string    `json:"channel"`
	Sender          string    `json:"sender"`
	Recipients      []string  `json:"recipients"`
	SentAt          time.Time `json:"sent_at"`
	Subject         string    `json:"subject"`
	Body            string    `json:"body"`
	Outbound        bool      `json:"outbound"`
	ListUnsubscribe bool      `json:"list_unsubscribe"`
	Status          string    `json:"status"`
	ExcludeReason   *string   `json:"exclude_reason"`
}

type Engagement struct {
	ID                int64      `json:"id"`
	EmetteurID        *int64     `json:"emetteur_id"`
	DestinataireID    *int64     `json:"destinataire_id"`
	EmetteurEmail     string     `json:"emetteur_email"`
	DestinataireEmail string     `json:"destinataire_email"`
	Objet             string     `json:"objet"`
	Type              string     `json:"type"`
	Echeance          *time.Time `json:"echeance"`
	EcheanceInferee   bool       `json:"echeance_inferee"`
	EcheanceConfirmee bool       `json:"echeance_confirmee"`
	Statut            string     `json:"statut"`
	Confiance         float64    `json:"confiance"`
	Priorite          string     `json:"priorite"`
	SourceMessageID   *int64     `json:"source_message_id"`
	ThreadID          *int64     `json:"thread_id"`
	CreeLe            time.Time  `json:"cree_le"`
	MajLe             time.Time  `json:"maj_le"`
}

type EngagementEvent struct {
	ID              int64           `json:"id"`
	EngagementID    int64           `json:"engagement_id"`
	Type            string          `json:"type"`
	Horodatage      time.Time       `json:"horodatage"`
	SourceMessageID *int64          `json:"source_message_id"`
	Details         json.RawMessage `json:"details"`
}

type DependencyLink struct {
	ID        int64     `json:"id"`
	AmontID   int64     `json:"amont_id"`
	AvalID    int64     `json:"aval_id"`
	Statut    string    `json:"statut"`
	Raison    string    `json:"raison"`
	CreatedAt time.Time `json:"created_at"`
	// jointures pour l'affichage
	AmontObjet string `json:"amont_objet,omitempty"`
	AvalObjet  string `json:"aval_objet,omitempty"`
}

type Capsule struct {
	Facts      json.RawMessage `json:"facts"`
	Intentions json.RawMessage `json:"intentions"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

type LearnedRule struct {
	ID          int64     `json:"id"`
	PorteeType  string    `json:"portee_type"`
	PorteeCible string    `json:"portee_cible"`
	Action      string    `json:"action"`
	Note        string    `json:"note"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
}

type Detection struct {
	ID           int64           `json:"id"`
	Type         string          `json:"type"`
	EngagementID *int64          `json:"engagement_id"`
	ThreadID     *int64          `json:"thread_id"`
	Score        float64         `json:"score"`
	Titre        string          `json:"titre"`
	Detail       string          `json:"detail"`
	Critique     bool            `json:"critique"`
	Payload      json.RawMessage `json:"payload"`
	Statut       string          `json:"statut"`
	CreatedAt    time.Time       `json:"created_at"`
}

type Draft struct {
	ID           int64      `json:"id"`
	DetectionID  *int64     `json:"detection_id"`
	EngagementID *int64     `json:"engagement_id"`
	ToEmail      string     `json:"to_email"`
	Subject      string     `json:"subject"`
	Body         string     `json:"body"`
	Statut       string     `json:"statut"`
	CreatedAt    time.Time  `json:"created_at"`
	SentAt       *time.Time `json:"sent_at"`
	// contexte pour l'affichage : pourquoi ce brouillon existe
	DetectionTitre  string `json:"detection_titre,omitempty"`
	DetectionDetail string `json:"detection_detail,omitempty"`
	EngagementObjet string `json:"engagement_objet,omitempty"`
}

type AuditEntry struct {
	ID        int64           `json:"id"`
	Ts        time.Time       `json:"ts"`
	Actor     string          `json:"actor"`
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
}

type Report struct {
	ID        int64           `json:"id"`
	Type      string          `json:"type"`
	Content   json.RawMessage `json:"content"`
	CreatedAt time.Time       `json:"created_at"`
}
