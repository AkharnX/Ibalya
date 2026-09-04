package store

import (
	"context"
	"os"
	"testing"
)

// TestIsolationTenant est le juge de paix du multi-utilisateur : il prouve que
// la base elle-même empêche un utilisateur de voir ou d'écrire les données d'un
// autre. Il tourne contre une vraie base (TEST_ADMIN_URL + TEST_APP_URL) ;
// sans elles, il est ignoré.
func TestIsolationTenant(t *testing.T) {
	adminURL := os.Getenv("TEST_ADMIN_URL")
	appURL := os.Getenv("TEST_APP_URL")
	if adminURL == "" || appURL == "" {
		t.Skip("TEST_ADMIN_URL / TEST_APP_URL non définis")
	}
	ctx := context.Background()
	st, err := New(ctx, adminURL, appURL)
	if err != nil {
		t.Fatalf("ouverture base: %v", err)
	}

	// Deux utilisateurs = deux tenants.
	a, err := st.CreateUser(ctx, "alice@test.fr", "Alice", "MotDePasseAlice123!")
	if err != nil {
		t.Fatalf("créer Alice: %v", err)
	}
	b, err := st.CreateUser(ctx, "bob@test.fr", "Bob", "MotDePasseBob123!")
	if err != nil {
		t.Fatalf("créer Bob: %v", err)
	}

	// Chacun crée une personne dans son tenant.
	mustTenant(t, st, a, func(ctx context.Context) error {
		_, e := st.UpsertPerson(ctx, "contact-a@ext.fr", "Contact de Alice")
		return e
	})
	mustTenant(t, st, b, func(ctx context.Context) error {
		_, e := st.UpsertPerson(ctx, "contact-b@ext.fr", "Contact de Bob")
		return e
	})

	// Alice ne doit voir QUE sa personne.
	mustTenant(t, st, a, func(ctx context.Context) error {
		ps, err := st.ListPersons(ctx)
		if err != nil {
			return err
		}
		if len(ps) != 1 || ps[0].Email != "contact-a@ext.fr" {
			t.Fatalf("Alice voit %d personne(s), attendu la sienne seule: %+v", len(ps), ps)
		}
		return nil
	})
	// Bob ne doit voir QUE la sienne.
	mustTenant(t, st, b, func(ctx context.Context) error {
		ps, err := st.ListPersons(ctx)
		if err != nil {
			return err
		}
		if len(ps) != 1 || ps[0].Email != "contact-b@ext.fr" {
			t.Fatalf("Bob voit %d personne(s), attendu la sienne seule: %+v", len(ps), ps)
		}
		return nil
	})

	// Hors tenant : la base ne montre rien (fermé par défaut).
	if ps, err := st.ListPersons(ctx); err == nil && len(ps) > 0 {
		t.Fatalf("hors tenant : %d personnes visibles, attendu 0", len(ps))
	}

	// Réglages : cloisonnés aussi.
	mustTenant(t, st, a, func(ctx context.Context) error { return st.SetSetting(ctx, "couleur", "bleu") })
	mustTenant(t, st, b, func(ctx context.Context) error { return st.SetSetting(ctx, "couleur", "rouge") })
	mustTenant(t, st, a, func(ctx context.Context) error {
		if v := st.GetSetting(ctx, "couleur", ""); v != "bleu" {
			t.Fatalf("réglage d'Alice = %q, attendu bleu (fuite ?)", v)
		}
		return nil
	})

	// Jetons OAuth : chacun connecte SA boîte, personne ne voit celle de l'autre.
	mustTenant(t, st, a, func(ctx context.Context) error {
		return st.SaveOAuthToken(ctx, "google", []byte(`{"access_token":"jeton-alice"}`), "alice@gmail.com")
	})
	mustTenant(t, st, b, func(ctx context.Context) error {
		return st.SaveOAuthToken(ctx, "google", []byte(`{"access_token":"jeton-bob"}`), "bob@gmail.com")
	})
	mustTenant(t, st, a, func(ctx context.Context) error {
		_, email, err := st.GetOAuthToken(ctx, "google")
		if err != nil {
			return err
		}
		if email != "alice@gmail.com" {
			t.Fatalf("Alice voit le compte %q, attendu le sien (fuite de boîte !)", email)
		}
		return nil
	})
	mustTenant(t, st, b, func(ctx context.Context) error {
		_, email, err := st.GetOAuthToken(ctx, "google")
		if err != nil {
			return err
		}
		if email != "bob@gmail.com" {
			t.Fatalf("Bob voit le compte %q, attendu le sien", email)
		}
		return nil
	})
}

func mustTenant(t *testing.T, st *Store, userID int64, fn func(context.Context) error) {
	t.Helper()
	if err := st.EnTenant(context.Background(), userID, fn); err != nil {
		t.Fatalf("EnTenant(%d): %v", userID, err)
	}
}
