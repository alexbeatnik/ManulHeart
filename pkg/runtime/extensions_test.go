package runtime

import (
	"context"
	"strings"
	"testing"

	"github.com/manulengineer/manulheart/pkg/browser"
	"github.com/manulengineer/manulheart/pkg/config"
	"github.com/manulengineer/manulheart/pkg/dsl"
	"github.com/manulengineer/manulheart/pkg/utils"
)

func TestCustomControlRegistryLookupIsCaseInsensitive(t *testing.T) {
	resetRuntimeRegistries()
	t.Cleanup(resetRuntimeRegistries)

	err := RegisterCustomControl("Login Page", "Username", func(context.Context, browser.Page, CustomControlInvocation) error {
		return nil
	})
	if err != nil {
		t.Fatalf("RegisterCustomControl failed: %v", err)
	}

	if _, ok := GetCustomControl("login page", "username"); !ok {
		t.Fatal("expected lowercase lookup to resolve registered custom control")
	}
	if _, ok := GetCustomControl("LOGIN PAGE", "USERNAME"); !ok {
		t.Fatal("expected uppercase lookup to resolve registered custom control")
	}
	if _, ok := GetCustomControl("Dashboard", "Username"); ok {
		t.Fatal("expected mismatched page lookup to return no custom control")
	}

	err = RegisterGoCall("Math.Concat", func(context.Context, GoCallInvocation) (any, error) {
		return nil, nil
	})
	if err != nil {
		t.Fatalf("RegisterGoCall failed: %v", err)
	}
	if _, ok := GetGoCall("math.concat"); !ok {
		t.Fatal("expected CALL GO lookup to be case-insensitive")
	}
}

func TestRuntime_CustomControlInterceptsFillWithoutDOMResolution(t *testing.T) {
	resetRuntimeRegistries()
	t.Cleanup(resetRuntimeRegistries)

	var gotAction string
	var gotValue string
	var gotPage string
	err := RegisterCustomControl("Checkout Page", "React Datepicker", func(ctx context.Context, page browser.Page, invocation CustomControlInvocation) error {
		gotAction = invocation.ActionType
		gotValue = invocation.Value
		gotPage = invocation.Page
		return nil
	})
	if err != nil {
		t.Fatalf("RegisterCustomControl failed: %v", err)
	}

	mock := &MockPage{URL: "https://example-shop.com/checkout", Title: "Checkout Page"}
	rt := New(config.Config{}, mock, utils.NewLogger(utils.LogLevelInfo, nil))

	res, execErr := rt.executeCommand(context.Background(), dsl.Command{
		Type:   dsl.CmdFill,
		Raw:    "FILL 'React Datepicker' field with '2026-12-25'",
		Target: "React Datepicker",
		Value:  "2026-12-25",
	})
	if execErr != nil {
		t.Fatalf("executeCommand failed: %v", execErr)
	}
	if gotAction != "input" {
		t.Fatalf("gotAction = %q, want input", gotAction)
	}
	if gotValue != "2026-12-25" {
		t.Fatalf("gotValue = %q, want 2026-12-25", gotValue)
	}
	if gotPage != "Checkout Page" {
		t.Fatalf("gotPage = %q, want Checkout Page", gotPage)
	}
	if mock.ProbeCalls != 0 {
		t.Fatalf("ProbeCalls = %d, want 0 because custom control should bypass DOM probing", mock.ProbeCalls)
	}
	if res.ProbeMetadata == nil || res.ProbeMetadata["resolution_strategy"] != "custom-control" {
		t.Fatalf("ProbeMetadata = %#v, want custom-control strategy", res.ProbeMetadata)
	}
	if !res.Success {
		t.Fatal("expected custom control execution to mark step successful")
	}
}

func TestRuntime_CustomControlFallsBackToURLDerivedPageLabel(t *testing.T) {
	resetRuntimeRegistries()
	t.Cleanup(resetRuntimeRegistries)

	called := false
	err := RegisterCustomControl("checkout page", "React Datepicker", func(ctx context.Context, page browser.Page, invocation CustomControlInvocation) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("RegisterCustomControl failed: %v", err)
	}

	mock := &MockPage{URL: "https://example-shop.com/checkout"}
	rt := New(config.Config{}, mock, utils.NewLogger(utils.LogLevelInfo, nil))

	_, execErr := rt.executeCommand(context.Background(), dsl.Command{
		Type:   dsl.CmdClick,
		Raw:    "CLICK the 'React Datepicker' element",
		Target: "React Datepicker",
	})
	if execErr != nil {
		t.Fatalf("executeCommand failed: %v", execErr)
	}
	if !called {
		t.Fatal("expected custom control lookup to fall back to URL-derived page label")
	}
	if mock.ProbeCalls != 0 {
		t.Fatalf("ProbeCalls = %d, want 0 because custom control should bypass DOM probing", mock.ProbeCalls)
	}
}

func TestRuntime_CallGoResolvesArgsAndStoresResult(t *testing.T) {
	resetRuntimeRegistries()
	t.Cleanup(resetRuntimeRegistries)

	var gotArgs []string
	err := RegisterGoCall("Math.Concat", func(ctx context.Context, invocation GoCallInvocation) (any, error) {
		gotArgs = append([]string(nil), invocation.Args...)
		if invocation.Variables["factor"] != "7" {
			t.Fatalf("variables[factor] = %q, want 7", invocation.Variables["factor"])
		}
		return strings.Join(invocation.Args, "-"), nil
	})
	if err != nil {
		t.Fatalf("RegisterGoCall failed: %v", err)
	}

	rt := New(config.Config{}, &MockPage{}, utils.NewLogger(utils.LogLevelInfo, nil))
	rt.vars.Set("factor", "7", LevelRow)

	res, execErr := rt.executeCommand(context.Background(), dsl.Command{
		Type:            dsl.CmdCallGo,
		Raw:             `CALL GO math.concat "6" {factor} into {product}`,
		GoCallName:      "math.concat",
		GoCallArgs:      []string{"6", "{factor}"},
		GoCallResultVar: "product",
	})
	if execErr != nil {
		t.Fatalf("executeCommand failed: %v", execErr)
	}
	wantArgs := []string{"6", "7"}
	if len(gotArgs) != len(wantArgs) {
		t.Fatalf("gotArgs len = %d, want %d (%v)", len(gotArgs), len(wantArgs), gotArgs)
	}
	for i := range wantArgs {
		if gotArgs[i] != wantArgs[i] {
			t.Fatalf("gotArgs[%d] = %q, want %q", i, gotArgs[i], wantArgs[i])
		}
	}
	if res.ActionValue != "6-7" {
		t.Fatalf("ActionValue = %q, want 6-7", res.ActionValue)
	}
	if value, ok := rt.vars.Resolve("product"); !ok || value != "6-7" {
		t.Fatalf("stored product = %q, ok=%v, want 6-7", value, ok)
	}
	if res.ProbeMetadata == nil || res.ProbeMetadata["resolution_strategy"] != "call-go" {
		t.Fatalf("ProbeMetadata = %#v, want call-go strategy", res.ProbeMetadata)
	}
	if !res.Success {
		t.Fatal("expected CALL GO execution to mark step successful")
	}
}
