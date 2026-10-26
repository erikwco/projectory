package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/erikwco/projectory/internal/app"
	"github.com/erikwco/projectory/internal/store"
)

func TestMissingEnterOffersRemovalWithoutSelecting(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	a := &app.App{Store: s}
	path := t.TempDir()
	if _, err := a.Add(ctx, app.AddInput{Path: path, Name: "Tienda", Label: "principal"}); err != nil {
		t.Fatal(err)
	}
	locations, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	b := NewBrowser(ctx, a, locations, false, "", "", "name", SettingsScreen{})
	b = update(b, press(tea.KeyEnter))
	if b.Mode != confirming || !strings.Contains(b.View().Content, "ya no existe") {
		t.Fatalf("Enter on a missing registered path must offer removal confirmation; mode=%v notice=%q", b.Mode, b.Notice)
	}
	if b.Choice != nil {
		t.Fatal("missing-path confirmation selected a location")
	}
}
