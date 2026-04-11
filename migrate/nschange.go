package migrate

import (
	"fmt"
	"strings"
)

// FormatNSChangeInstructions returns ASCII, copy-pasteable instructions for
// changing nameservers at the source registrar (D-20, D-21). The caller passes
// the detected source slug (from entree.DetectProvider) and the new NS list
// assigned by the target provider. Unknown slugs get a generic block.
func FormatNSChangeInstructions(sourceSlug string, newNS []string) string {
	var b strings.Builder

	b.WriteString("Nameserver change required\n")
	b.WriteString("==========================\n\n")

	switch strings.ToLower(sourceSlug) {
	case "godaddy":
		b.WriteString("Source registrar: GoDaddy\n")
		b.WriteString("1. Sign in at https://dcc.godaddy.com/control/portfolio\n")
		b.WriteString("2. Select the domain and click 'Manage DNS'\n")
		b.WriteString("3. Under 'Nameservers' click 'Change' and choose 'I'll use my own nameservers'\n")
		b.WriteString("4. Paste the nameservers below and save\n")
	case "cloudflare":
		b.WriteString("Source registrar: Cloudflare\n")
		b.WriteString("1. Sign in at https://dash.cloudflare.com\n")
		b.WriteString("2. Select the domain, then go to DNS > Records\n")
		b.WriteString("3. Under 'Nameservers' click 'Change' and set custom nameservers\n")
	case "route53", "amazon", "aws":
		b.WriteString("Source registrar: Route 53 / Amazon Registrar\n")
		b.WriteString("1. Sign in at https://console.aws.amazon.com/route53/domains\n")
		b.WriteString("2. Select the domain and click 'Add or edit name servers'\n")
		b.WriteString("3. Replace with the nameservers below and save\n")
	case "google_cloud_dns", "google":
		b.WriteString("Source registrar: Google Domains / Squarespace\n")
		b.WriteString("1. Sign in at https://domains.squarespace.com\n")
		b.WriteString("2. Select the domain and open 'DNS'\n")
		b.WriteString("3. Switch to custom nameservers and paste the list below\n")
	case "namecheap":
		b.WriteString("Source registrar: Namecheap\n")
		b.WriteString("1. Sign in at https://ap.www.namecheap.com/Domains/DomainControlPanel\n")
		b.WriteString("2. Select the domain and set 'Nameservers' to 'Custom DNS'\n")
		b.WriteString("3. Paste the nameservers below and save\n")
	default:
		label := sourceSlug
		if label == "" {
			label = "unknown"
		}
		b.WriteString(fmt.Sprintf("Source registrar: %s\n", label))
		b.WriteString("At your registrar's control panel, replace the current nameservers\n")
		b.WriteString("with the list below. The change may take up to 48 hours to propagate.\n")
	}

	b.WriteString("\nNew nameservers:\n")
	if len(newNS) == 0 {
		b.WriteString("  (no nameservers returned by the target provider)\n")
	} else {
		for _, ns := range newNS {
			b.WriteString("  " + strings.TrimSuffix(ns, ".") + "\n")
		}
	}
	b.WriteString("\nVerification: run 'dig NS <domain>' and confirm the new nameservers appear.\n")
	return b.String()
}
