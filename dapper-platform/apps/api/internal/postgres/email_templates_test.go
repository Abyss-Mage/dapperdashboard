package postgres

import (
	"strings"
	"testing"
)

func TestRenderEmailFillsPlaceholdersAndLinksTheLink(t *testing.T) {
	subject, htmlBody, text := renderEmail("Hi {{name}}", "Open {{link}}\n\nBye <{{name}}>", false, map[string]string{"name": "Ann & Bo", "link": "https://x.test/r?t=1&u=2"})
	if subject != "Hi Ann & Bo" || !strings.Contains(text, "Open https://x.test/r?t=1&u=2") {
		t.Fatalf("plain text wrong: %q / %q", subject, text)
	}
	if !strings.Contains(htmlBody, `<a href="https://x.test/r?t=1&amp;u=2">`) || !strings.Contains(htmlBody, "Bye &lt;Ann &amp; Bo&gt;") {
		t.Fatalf("html must link the link and escape the values: %s", htmlBody)
	}
}

func TestHTMLEmailEscapesValuesAndHasATextTwin(t *testing.T) {
	_, htmlBody, text := renderEmail("S", "<p>Hi <b>{{name}}</b></p><p>Bye</p>", true, map[string]string{"name": "A<b>"})
	if !strings.Contains(htmlBody, "<b>A&lt;b&gt;</b>") || text != "Hi A<b>\nBye" {
		t.Fatalf("html email wrong: %q / %q", htmlBody, text)
	}
}

func TestEveryBuiltInEmailKeepsItsRequiredLink(t *testing.T) {
	for _, k := range EmailKeys {
		d := emailDefaults[k]
		if d.needsLink && !strings.Contains(d.body, "{{link}}") {
			t.Fatalf("%s lost its link", k)
		}
	}
}
