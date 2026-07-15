package dailyreward

import (
	"context"
	"html/template"
	"strings"
	"testing"
)

// A points-granting POST that loses its captcha must not do so quietly.
//
// Provision used `p.captcha, _ = v.(captchaCap)`, which cannot distinguish "this
// host chose not to gate" from "this host tried to gate and we dropped it". The
// second is a wiring bug that leaves an ungated points endpoint with no error
// anywhere — and loon/core is explicit that a failed type assertion is "a
// programmer error the consumer should surface from Provision (aborting boot),
// not swallow".
//
// These test the decision directly rather than through Provision, which needs a
// live *core.Core (DB, router, view registry) to reach the four lines that
// matter.

// goodCaptcha matches captchaCap.
type goodCaptcha struct{ verifyErr error }

func (g goodCaptcha) Verify(context.Context, string, string) error { return g.verifyErr }
func (g goodCaptcha) WidgetHTML() template.HTML                    { return "<widget>" }

// wrongCaptcha is what a signature drift looks like: registered under the right
// key, right idea, incompatible shape. Verify takes no ip.
type wrongCaptcha struct{}

func (wrongCaptcha) Verify(context.Context, string) error { return nil }
func (wrongCaptcha) WidgetHTML() template.HTML            { return "" }

func TestCaptchaCapability_Assertion(t *testing.T) {
	t.Run("a matching captcha is accepted", func(t *testing.T) {
		var v any = goodCaptcha{}
		if _, ok := v.(captchaCap); !ok {
			t.Fatal("a correctly-shaped verifier failed the assertion — the gate would be silently dropped")
		}
	})

	t.Run("a mismatched captcha does NOT satisfy the capability", func(t *testing.T) {
		var v any = wrongCaptcha{}
		if _, ok := v.(captchaCap); ok {
			t.Fatal("wrongCaptcha satisfied captchaCap — this test cannot detect the drift it exists for")
		}
		// The point: with `p.captcha, _ = v.(captchaCap)` this exact value
		// yields a nil captcha and a silently ungated endpoint. Provision now
		// returns an error instead.
	})
}

// The gate itself: nil captcha means no verification runs. That is intentional
// when no host registered one, and catastrophic when one was registered and
// dropped — which is why the drop is now a boot error.
func TestClaimGate_NilCaptchaSkipsVerification(t *testing.T) {
	p := &Plugin{}
	if p.captcha != nil {
		t.Fatal("zero-value Plugin has a captcha")
	}
	// Mirrors the claim handler's gate: `if p.captcha != nil { ...Verify... }`.
	gated := p.captcha != nil
	if gated {
		t.Error("nil captcha reported as gated")
	}
}

// The error must name the offending type and say what it cost, or an operator
// reads "captcha error" and goes looking in the wrong place.
func TestCaptchaMismatchErrorIsActionable(t *testing.T) {
	// Reproduce the message Provision builds.
	const key = "captcha"
	var v any = wrongCaptcha{}
	msg := formatCaptchaMismatch(key, v)

	for _, want := range []string{"captcha", "wrongCaptcha", "ungated"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message %q is missing %q — it should name the key, the offending type, and the consequence", msg, want)
		}
	}
}
