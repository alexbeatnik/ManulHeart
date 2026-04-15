package dom

// EcommerceDOM returns the 100-element eCommerce DOM snapshot used for integration testing.
func EcommerceDOM() []ElementSnapshot {
	// Helper for building snapshots
	el := func(idx int, xpath string, opts ...func(*ElementSnapshot)) ElementSnapshot {
		e := ElementSnapshot{
			ID:        idx,
			XPath:     xpath,
			Tag:       "div",
			IsVisible: true,
			Rect:      Rect{Top: float64(idx * 40), Left: 10, Width: 100, Height: 30},
		}
		for _, o := range opts {
			o(&e)
		}
		e.Normalize()
		return e
	}
	
	// Property helpers
	withTag := func(tag string) func(*ElementSnapshot) { return func(e *ElementSnapshot) { e.Tag = tag } }
	withID := func(id string) func(*ElementSnapshot) { return func(e *ElementSnapshot) { e.HTMLId = id } }
	withText := func(text string) func(*ElementSnapshot) { return func(e *ElementSnapshot) { e.VisibleText = text } }
	withRole := func(role string) func(*ElementSnapshot) { return func(e *ElementSnapshot) { e.Role = role } }
	withAriaLabel := func(a string) func(*ElementSnapshot) { return func(e *ElementSnapshot) { e.AriaLabel = a } }
	withClassName := func(c string) func(*ElementSnapshot) { return func(e *ElementSnapshot) { e.ClassName = c } }
	withInputType := func(t string) func(*ElementSnapshot) { return func(e *ElementSnapshot) { e.InputType = t } }
	withValue := func(v string) func(*ElementSnapshot) { return func(e *ElementSnapshot) { e.Value = v } }
	withDisabled := func() func(*ElementSnapshot) { return func(e *ElementSnapshot) { e.IsDisabled = true } }
	withAccessibleName := func(n string) func(*ElementSnapshot) { return func(e *ElementSnapshot) { e.AccessibleName = n } }

	return []ElementSnapshot{
		// ── Cart Buttons (e1-e10) ─────────────────────────────
		el(1, "/html/body/button[1]", withTag("button"), withID("e1"), withText("Add to Cart")),
		el(2, "/html/body/div[1]", withTag("div"), withRole("button"), withID("e2"), withAriaLabel("Add to Bag"), withText("🛒")),
		el(3, "/html/body/a[1]", withTag("a"), withID("e3"), withClassName("btn"), withText("Add to Basket")),
		el(4, "/html/body/button[2]", withTag("button"), withID("e4"), withText("Add Product"), withAccessibleName("Add Product")),
		el(5, "/html/body/input[1]", withTag("input"), withInputType("submit"), withID("e5"), withValue("Buy Now")),
		el(6, "/html/body/button[3]", withTag("button"), withID("e6"), withText("Add to Cart"), withDisabled()),
		el(7, "/html/body/button[4]", withTag("button"), withID("e7"), withText("Pre-order")),
		el(8, "/html/body/div[2]", withTag("div"), withID("e8"), withText("Toss in Cart")),
		el(9, "/html/body/button[5]", withTag("button"), withID("e9"), withText("Add")),
		el(10, "/html/body/button[6]", withTag("button"), withID("e10"), withClassName("add-to-cart-action"), withText("Submit")),
	}
}

// SocialDOM returns a social media DOM snapshot.
func SocialDOM() []ElementSnapshot {
	el := func(idx int, xpath string, opts ...func(*ElementSnapshot)) ElementSnapshot {
		e := ElementSnapshot{
			ID: idx, XPath: xpath, Tag: "div", IsVisible: true,
			Rect: Rect{Top: float64(idx * 40), Left: 10, Width: 100, Height: 30},
		}
		for _, o := range opts { o(&e) }
		e.Normalize()
		return e
	}
	withID := func(id string) func(*ElementSnapshot) { return func(e *ElementSnapshot) { e.HTMLId = id } }
	withText := func(text string) func(*ElementSnapshot) { return func(e *ElementSnapshot) { e.VisibleText = text } }
	withTag := func(tag string) func(*ElementSnapshot) { return func(e *ElementSnapshot) { e.Tag = tag } }

	return []ElementSnapshot{
		el(1, "/body/div[1]", withID("post1"), withText("Create Post")),
		el(2, "/body/div[2]", withID("like-btn"), withText("Like")),
		el(3, "/body/div[3]", withID("comment-box"), withTag("textarea"), withText("Write a comment...")),
	}
}

// TravelDOM returns a travel booking DOM snapshot.
func TravelDOM() []ElementSnapshot {
	el := func(idx int, xpath string, opts ...func(*ElementSnapshot)) ElementSnapshot {
		e := ElementSnapshot{
			ID: idx, XPath: xpath, Tag: "div", IsVisible: true,
			Rect: Rect{Top: float64(idx * 40), Left: 10, Width: 100, Height: 30},
		}
		for _, o := range opts { o(&e) }
		e.Normalize()
		return e
	}
	withID := func(id string) func(*ElementSnapshot) { return func(e *ElementSnapshot) { e.HTMLId = id } }
	withText := func(text string) func(*ElementSnapshot) { return func(e *ElementSnapshot) { e.VisibleText = text } }

	return []ElementSnapshot{
		el(1, "/body/div[1]", withID("search-flights"), withText("Search Flights")),
		el(2, "/body/div[2]", withID("dest-input"), withText("Enter destination")),
	}
}

// SaasDOM returns a SaaS admin dashboard DOM snapshot.
func SaasDOM() []ElementSnapshot {
	el := func(idx int, xpath string, opts ...func(*ElementSnapshot)) ElementSnapshot {
		e := ElementSnapshot{
			ID: idx, XPath: xpath, Tag: "div", IsVisible: true,
			Rect: Rect{Top: float64(idx * 40), Left: 10, Width: 100, Height: 30},
		}
		for _, o := range opts { o(&e) }
		e.Normalize()
		return e
	}
	withID := func(id string) func(*ElementSnapshot) { return func(e *ElementSnapshot) { e.HTMLId = id } }
	withText := func(text string) func(*ElementSnapshot) { return func(e *ElementSnapshot) { e.VisibleText = text } }

	return []ElementSnapshot{
		el(1, "/body/nav/a[1]", withID("nav-users"), withText("Users")),
		el(2, "/body/nav/a[2]", withID("nav-billing"), withText("Billing")),
		el(3, "/body/button[1]", withID("add-user"), withText("Add User")),
	}
}

// MediaDOM returns a media player DOM snapshot.
func MediaDOM() []ElementSnapshot {
	el := func(idx int, xpath string, opts ...func(*ElementSnapshot)) ElementSnapshot {
		e := ElementSnapshot{
			ID: idx, XPath: xpath, Tag: "div", IsVisible: true,
			Rect: Rect{Top: float64(idx * 40), Left: 10, Width: 100, Height: 30},
		}
		for _, o := range opts { o(&e) }
		e.Normalize()
		return e
	}
	withID := func(id string) func(*ElementSnapshot) { return func(e *ElementSnapshot) { e.HTMLId = id } }
	withText := func(text string) func(*ElementSnapshot) { return func(e *ElementSnapshot) { e.VisibleText = text } }

	return []ElementSnapshot{
		el(1, "/body/div[1]", withID("play-btn"), withText("Play")),
		el(2, "/body/div[2]", withID("pause-btn"), withText("Pause")),
		el(3, "/body/div[3]", withID("mute-btn"), withText("Mute")),
	}
}
