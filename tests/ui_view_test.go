//go:build browser

package tests

import (
	"testing"

	"github.com/veltylabs/payment_manager/ui"
	"webtyp.com/model"
	"webtyp.com/router"
)

type mockCaller struct{}

func (c *mockCaller) Call(route string, args model.Encodable, res model.Decodable, cb func(error)) {
	if cb != nil {
		cb(nil)
	}
}

func (c *mockCaller) Dispatch(route string, args model.Encodable) {}

var _ router.Caller = (*mockCaller)(nil)

func TestUI_Screens(t *testing.T) {
	fake := &mockCaller{}
	screens := ui.Screens(fake)
	if len(screens) != 1 {
		t.Fatalf("len(screens) = %d, want 1", len(screens))
	}
	s := screens[0]
	if err := s.Validate(); err != nil {
		t.Fatalf("screen.Validate() error: %v", err)
	}
	if s.ID != ui.ID {
		t.Errorf("ID = %q, want %q", s.ID, ui.ID)
	}
	if s.Presenter.Description() == "" {
		t.Error("Description() is empty")
	}
}
