//go:build e2e

package e2e_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/mxschmitt/playwright-go"
)

var (
	baseURL = "http://localhost:8080"
)

func init() {
	if url := os.Getenv("BASE_URL"); url != "" {
		baseURL = url
	}
	if url := os.Getenv("APP_BASE_URL"); url != "" {
		baseURL = url
	}
}

func setupPlaywright(t *testing.T) (*playwright.Playwright, playwright.Browser, playwright.BrowserContext, playwright.Page) {
	err := playwright.Install()
	if err != nil {
		t.Fatalf("could not install playwright: %v", err)
	}

	pw, err := playwright.Run()
	if err != nil {
		t.Fatalf("could not start playwright: %v", err)
	}

	browser, err := pw.Chromium.Launch()
	if err != nil {
		t.Fatalf("could not launch browser: %v", err)
	}

	context, err := browser.NewContext()
	if err != nil {
		t.Fatalf("could not create context: %v", err)
	}

	page, err := context.NewPage()
	if err != nil {
		t.Fatalf("could not create page: %v", err)
	}

	return pw, browser, context, page
}

func teardownPlaywright(t *testing.T, pw *playwright.Playwright, browser playwright.Browser, context playwright.BrowserContext, page playwright.Page) {
	if err := page.Close(); err != nil {
		t.Logf("could not close page: %v", err)
	}
	if err := context.Close(); err != nil {
		t.Logf("could not close context: %v", err)
	}
	if err := browser.Close(); err != nil {
		t.Logf("could not close browser: %v", err)
	}
	if err := pw.Stop(); err != nil {
		t.Logf("could not stop Playwright: %v", err)
	}
}

func TestSubscriptionFlow(t *testing.T) {
	pw, browser, context, page := setupPlaywright(t)
	defer teardownPlaywright(t, pw, browser, context, page)

	expect := playwright.NewPlaywrightAssertions()

	testEmail := "test-e2e@example.com"
	testRepo := "golang/go"
	testAPIKey := "test-api-key-12345"

	t.Run("should have the correct title", func(t *testing.T) {
		if _, err := page.Goto(baseURL + "/"); err != nil {
			t.Fatalf("could not goto: %v", err)
		}
		title, err := page.Title()
		if err != nil {
			t.Fatalf("could not get title: %v", err)
		}
		if !strings.Contains(title, "Release Watch") {
			t.Errorf("expected title to contain 'Release Watch', got %s", title)
		}

		h1 := page.Locator("h1")
		if isVisible, _ := h1.IsVisible(); !isVisible {
			t.Errorf("expected h1 to be visible")
		}
		if text, _ := h1.TextContent(); text != "Release Watch" {
			t.Errorf("expected h1 text to be 'Release Watch', got %s", text)
		}
	})

	t.Run("should allow saving an API key", func(t *testing.T) {
		if _, err := page.Goto(baseURL + "/"); err != nil {
			t.Fatalf("could not goto: %v", err)
		}

		apiKeyInput := page.Locator("#api-key")
		saveButton := page.Locator("#save-api-key")
		status := page.Locator("#api-key-status")

		if err := apiKeyInput.Fill(testAPIKey); err != nil {
			t.Fatalf("could not fill api key: %v", err)
		}
		if err := saveButton.Click(); err != nil {
			t.Fatalf("could not click save: %v", err)
		}

		if err := expect.Locator(status).ToHaveText("API key saved."); err != nil {
			t.Errorf("status text mismatch: %v", err)
		}
		if err := expect.Locator(status).ToHaveClass(regexp.MustCompile(`ok`)); err != nil {
			t.Errorf("status class mismatch: %v", err)
		}

		if _, err := page.Reload(); err != nil {
			t.Fatalf("could not reload: %v", err)
		}

		if err := expect.Locator(apiKeyInput).ToHaveValue(testAPIKey); err != nil {
			t.Errorf("apiKeyInput value mismatch: %v", err)
		}
		if err := expect.Locator(status).ToHaveText("Using saved API key."); err != nil {
			t.Errorf("status text mismatch after reload: %v", err)
		}
	})

	t.Run("should show validation errors for empty subscription form", func(t *testing.T) {
		if _, err := page.Goto(baseURL + "/"); err != nil {
			t.Fatalf("could not goto: %v", err)
		}

		submitButton := page.Locator("#subscribe-form button[type=\"submit\"]")
		if err := submitButton.Click(); err != nil {
			t.Fatalf("could not click submit: %v", err)
		}

		status := page.Locator("#subscribe-status")
		if err := expect.Locator(status).ToBeEmpty(); err != nil {
			t.Errorf("status should be empty: %v", err)
		}
	})

	t.Run("should handle a successful subscription request", func(t *testing.T) {
		if _, err := page.Goto(baseURL + "/"); err != nil {
			t.Fatalf("could not goto: %v", err)
		}

		if err := page.Locator("#api-key").Fill(testAPIKey); err != nil {
			t.Fatalf("could not fill api key: %v", err)
		}
		if err := page.Locator("#save-api-key").Click(); err != nil {
			t.Fatalf("could not click save: %v", err)
		}

		if err := page.Locator("#subscribe-email").Fill(testEmail); err != nil {
			t.Fatalf("could not fill email: %v", err)
		}
		if err := page.Locator("#subscribe-repo").Fill(testRepo); err != nil {
			t.Fatalf("could not fill repo: %v", err)
		}

		err := context.Route("**/api/subscribe", func(route playwright.Route) {
			route.Fulfill(playwright.RouteFulfillOptions{
				Status:      playwright.Int(200),
				ContentType: playwright.String("application/json"),
				Body:        playwright.String(`{"message": "subscription created; confirmation email sent"}`),
			})
		})
		if err != nil {
			t.Fatalf("could not route: %v", err)
		}

		if err := page.Locator("#subscribe-form button[type=\"submit\"]").Click(); err != nil {
			t.Fatalf("could not click submit: %v", err)
		}

		status := page.Locator("#subscribe-status")
		if err := expect.Locator(status).ToHaveText(regexp.MustCompile(`subscription created`)); err != nil {
			t.Errorf("status text mismatch: %v", err)
		}
		if err := expect.Locator(status).ToHaveClass(regexp.MustCompile(`ok`)); err != nil {
			t.Errorf("status class mismatch: %v", err)
		}

		if err := expect.Locator(page.Locator("#lookup-email")).ToHaveValue(testEmail); err != nil {
			t.Errorf("lookup email value mismatch: %v", err)
		}
	})

	t.Run("should handle loading subscriptions", func(t *testing.T) {
		if _, err := page.Goto(baseURL + "/"); err != nil {
			t.Fatalf("could not goto: %v", err)
		}

		if err := page.Locator("#api-key").Fill(testAPIKey); err != nil {
			t.Fatalf("could not fill api key: %v", err)
		}
		if err := page.Locator("#save-api-key").Click(); err != nil {
			t.Fatalf("could not click save: %v", err)
		}

		if err := page.Locator("#lookup-email").Fill(testEmail); err != nil {
			t.Fatalf("could not fill lookup email: %v", err)
		}

		err := context.Route("**/ui/subscriptions**", func(route playwright.Route) {
			route.Fulfill(playwright.RouteFulfillOptions{
				Status:      playwright.Int(200),
				ContentType: playwright.String("application/json"),
				Body: playwright.String(`[
					{
						"email": "test-e2e@example.com",
						"repo": "golang/go",
						"confirmed": true,
						"last_seen_tag": "v1.22.0",
						"unsubscribe_token": "test-token-123"
					}
				]`),
			})
		})
		if err != nil {
			t.Fatalf("could not route: %v", err)
		}

		if err := page.Locator("#lookup-form button[type=\"submit\"]").Click(); err != nil {
			t.Fatalf("could not click submit: %v", err)
		}

		results := page.Locator("#lookup-results")
		if err := expect.Locator(results.Locator(".subscription")).ToBeVisible(); err != nil {
			t.Errorf("subscription should be visible: %v", err)
		}
		if err := expect.Locator(results.Locator(".repo")).ToHaveText(testRepo); err != nil {
			t.Errorf("repo text mismatch: %v", err)
		}
		if err := expect.Locator(results.Locator(".meta").First()).ToHaveText(regexp.MustCompile(`Confirmed: yes`)); err != nil {
			t.Errorf("meta text mismatch: %v", err)
		}
	})
}
