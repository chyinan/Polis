package pdf

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/giraffesyo/pdf/pdftest"
)

func TestExtractContentOperatorAuditIncludesPageAndFormStreams(t *testing.T) {
	content := "q 1 0 0 1 3 4 cm 0 0 m 12 12 l S /Fm Do BT /F1 12 Tf (Hi) Tj ET Q"
	form := pdftest.Stream("/Type /XObject /Subtype /Form /BBox [0 0 10 10]", "0 0 re f")
	pdfBytes := pdftest.Build(1,
		pdftest.Catalog(2),
		pdftest.Pages(3),
		pdftest.Page(2, 4, "<< /Font << /F1 5 0 R >> /XObject << /Fm 6 0 R >> >>"),
		pdftest.Stream("", content),
		pdftest.Helvetica(),
		form,
	)

	document, err := ExtractWithOptions(context.Background(), bytes.NewReader(pdfBytes), int64(len(pdfBytes)), Options{
		AuditContentOperators: true,
		Concurrency:           1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Pages) != 1 {
		t.Fatalf("page count=%d, want 1", len(document.Pages))
	}
	page := document.Pages[0]
	want := []string{"BT", "Do", "ET", "Q", "S", "Tf", "Tj", "cm", "f", "l", "m", "q", "re"}
	if !page.ContentOperatorNamesComplete || !reflect.DeepEqual(page.ContentOperatorNames, want) {
		t.Fatalf("operator audit complete=%t names=%v, want complete and %v", page.ContentOperatorNamesComplete, page.ContentOperatorNames, want)
	}
	if got := page.Text(); got != "Hi" {
		t.Fatalf("audit changed existing extraction result: text=%q, want %q", got, "Hi")
	}
}

func TestExtractContentOperatorAuditCanBeDisabled(t *testing.T) {
	pdfBytes := pdftest.Build(1,
		pdftest.Catalog(2), pdftest.Pages(3),
		pdftest.Page(2, 4, "<< >>"), pdftest.Stream("", "0 0 m 1 1 l S"),
	)
	document, err := Extract(context.Background(), bytes.NewReader(pdfBytes), int64(len(pdfBytes)))
	if err != nil {
		t.Fatal(err)
	}
	page := document.Pages[0]
	if page.ContentOperatorNames != nil || page.ContentOperatorNamesComplete {
		t.Fatalf("disabled audit populated fields: complete=%t names=%v", page.ContentOperatorNamesComplete, page.ContentOperatorNames)
	}
}

func TestExtractContentOperatorAuditMarksMalformedOrOverBudgetStreamsIncomplete(t *testing.T) {
	t.Run("malformed operator", func(t *testing.T) {
		pdfBytes := pdftest.Build(1,
			pdftest.Catalog(2), pdftest.Pages(3),
			pdftest.Page(2, 4, "<< >>"), pdftest.Stream("", "0 0 m Q"),
		)
		document, err := ExtractWithOptions(context.Background(), bytes.NewReader(pdfBytes), int64(len(pdfBytes)), Options{AuditContentOperators: true, Concurrency: 1})
		if err != nil {
			t.Fatal(err)
		}
		if document.Pages[0].ContentOperatorNamesComplete {
			t.Fatal("malformed content stream was reported as a complete operator audit")
		}
	})

	t.Run("unique name limit", func(t *testing.T) {
		var content strings.Builder
		for index := 0; index < 257; index++ {
			fmt.Fprintf(&content, "x%03d ", index)
		}
		pdfBytes := pdftest.Build(1,
			pdftest.Catalog(2), pdftest.Pages(3),
			pdftest.Page(2, 4, "<< >>"), pdftest.Stream("", content.String()),
		)
		document, err := ExtractWithOptions(context.Background(), bytes.NewReader(pdfBytes), int64(len(pdfBytes)), Options{AuditContentOperators: true, Concurrency: 1})
		if err != nil {
			t.Fatal(err)
		}
		page := document.Pages[0]
		if page.ContentOperatorNamesComplete || len(page.ContentOperatorNames) != 256 {
			t.Fatalf("unique-name overflow audit complete=%t names=%d, want incomplete and capped at 256", page.ContentOperatorNamesComplete, len(page.ContentOperatorNames))
		}
	})

	t.Run("operator name length limit", func(t *testing.T) {
		name := strings.Repeat("x", 129)
		pdfBytes := pdftest.Build(1,
			pdftest.Catalog(2), pdftest.Pages(3),
			pdftest.Page(2, 4, "<< >>"), pdftest.Stream("", name),
		)
		document, err := ExtractWithOptions(context.Background(), bytes.NewReader(pdfBytes), int64(len(pdfBytes)), Options{AuditContentOperators: true, Concurrency: 1})
		if err != nil {
			t.Fatal(err)
		}
		if document.Pages[0].ContentOperatorNamesComplete {
			t.Fatal("overlong operator name was reported as a complete audit")
		}
	})
}
