package payouts

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"lenapay/apps/api/internal/platform/money"
	"lenapay/apps/api/internal/psp"
)

// The proof of payment (R-OUT-6) is a one-page PDF written by hand: the standard Helvetica
// fonts need no embedding, so a few objects and an xref table are the whole format, and the
// API takes no PDF dependency. Text is WinAnsi (Latin-1), which covers es-AR.

// proofCopy is the es-AR copy of the proof, in one place (G13).
func proofCopy() map[string]string {
	return map[string]string{
		"title":       "Comprobante de transferencia",
		"status":      "Estado",
		"statusValue": "Acreditada",
		"date":        "Fecha",
		"amount":      "Importe",
		"fee":         "Comisión",
		"total":       "Total debitado",
		"holder":      "Destinatario",
		"cuit":        "CUIT/CUIL",
		"account":     "CBU/CVU",
		"alias":       "Alias",
		"bank":        "Banco",
		"concept":     "Concepto",
		"operation":   "N.º de operación",
		"reference":   "Referencia del banco",
		"footer":      "Lenapay · Tus cobros y pagos, al instante.",
	}
}

// Brand colors (docs/brand.md) as PDF RGB fractions.
const (
	colorPrimary   = "0.071 0.357 1"     // #125BFF
	colorText      = "0.094 0.102 0.129" // #181A21
	colorSecondary = "0.314 0.337 0.392" // #505664
)

type proofRow struct{ label, value string }

// renderProof renders a settled payout; times are shown in loc (the business time zone).
func renderProof(po Payout, loc *time.Location) ([]byte, error) {
	total, err := po.Total()
	if err != nil {
		return nil, fmt.Errorf("payouts: proof total: %w", err)
	}
	t := proofCopy()
	settled := po.UpdatedAt
	if po.SettledAt != nil {
		settled = *po.SettledAt
	}
	rows := []proofRow{
		{t["status"], t["statusValue"]},
		{t["date"], settled.In(loc).Format("02/01/2006 15:04")},
		{t["amount"], formatMoney(po.Amount)},
		{t["fee"], formatMoney(po.Fee)},
		{t["total"], formatMoney(total)},
		{t["holder"], po.Destination.HolderName},
		{t["cuit"], formatCUIT(po.Destination.HolderCUIT)},
		{t["account"], po.Destination.AccountNumber},
	}
	if po.Destination.Type == psp.DestinationAlias {
		rows = append(rows, proofRow{t["alias"], po.Destination.Value})
	}
	if po.Destination.BankName != "" {
		rows = append(rows, proofRow{t["bank"], po.Destination.BankName})
	}
	rows = append(rows, proofRow{t["concept"], po.Concept}, proofRow{t["operation"], po.ID.String()})
	if po.ProviderTransferID != nil {
		rows = append(rows, proofRow{t["reference"], *po.ProviderTransferID})
	}
	return buildPDF(proofContent(t["title"], t["footer"], rows)), nil
}

func proofContent(title, footer string, rows []proofRow) string {
	var c strings.Builder
	fmt.Fprintf(&c, "%s rg 0 752 595 90 re f\n", colorPrimary)
	fmt.Fprintf(&c, "BT /F2 24 Tf 1 1 1 rg 40 800 Td (%s) Tj ET\n", pdfText("Lenapay"))
	fmt.Fprintf(&c, "BT /F1 13 Tf 1 1 1 rg 40 776 Td (%s) Tj ET\n", pdfText(title))
	y := 712
	for _, r := range rows {
		fmt.Fprintf(&c, "BT /F1 10 Tf %s rg 40 %d Td (%s) Tj ET\n", colorSecondary, y, pdfText(r.label))
		fmt.Fprintf(&c, "BT /F2 11 Tf %s rg 200 %d Td (%s) Tj ET\n", colorText, y, pdfText(r.value))
		y -= 24
	}
	fmt.Fprintf(&c, "BT /F1 9 Tf %s rg 40 48 Td (%s) Tj ET\n", colorSecondary, pdfText(footer))
	return c.String()
}

// buildPDF writes the five objects of a one-page A4 document and its cross-reference table.
func buildPDF(content string) []byte {
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] " +
			"/Resources << /Font << /F1 4 0 R /F2 5 0 R >> >> /Contents 6 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	offsets := make([]int, len(objects))
	for i, o := range objects {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return b.Bytes()
}

// pdfText encodes s as a WinAnsi literal string: Latin-1 bytes, with the delimiters escaped;
// characters outside Latin-1 become "?" and the middle dot is kept (0xB7).
func pdfText(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\\' || r == '(' || r == ')':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r < 0x20:
			b.WriteByte(' ')
		case r < 0x80:
			b.WriteRune(r)
		case r <= 0xFF:
			fmt.Fprintf(&b, "\\%03o", r)
		default:
			b.WriteByte('?')
		}
	}
	return b.String()
}

// formatMoney renders minor units as es-AR money: "$ 1.500,00" or "US$ 1.500,00" (docs/brand.md).
func formatMoney(a money.Amount) string {
	prefix := "$ "
	if a.Currency == money.USD {
		prefix = "US$ "
	}
	minor := a.Minor
	sign := ""
	if minor < 0 {
		sign, minor = "-", -minor
	}
	units := strconv.FormatInt(minor/100, 10)
	var grouped strings.Builder
	for i, d := range units {
		if i > 0 && (len(units)-i)%3 == 0 {
			grouped.WriteByte('.')
		}
		grouped.WriteRune(d)
	}
	return fmt.Sprintf("%s%s%s,%02d", sign, prefix, grouped.String(), minor%100)
}

// formatCUIT renders 11 digits as XX-XXXXXXXX-X.
func formatCUIT(cuit string) string {
	if len(cuit) != 11 {
		return cuit
	}
	return cuit[:2] + "-" + cuit[2:10] + "-" + cuit[10:]
}
