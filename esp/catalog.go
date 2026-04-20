package esp

import "strings"

// Catalog maps SPF include hosts (lowercased, no leading "include:") to
// ESPInfo entries. Used as the primary classification lookup during SPF
// analysis.
//
// Entries are grouped by infrastructure:
//   - InfraSES: senders that run on Amazon SES, whether direct or via an ESP
//   - Other ESPs that have their own infrastructure (SendGrid, Postmark, etc.)
//   - Mailbox providers (Google Workspace, M365, Zoho, etc.)
//
// When adding new entries, include BOTH SPF include aliases a provider may
// publish (providers sometimes have legacy + current include hosts).
var Catalog = map[string]ESPInfo{
	// ---- SES-backed ESPs -------------------------------------------------
	// Customer does not touch AWS; we integrate via the ESP's API but flag
	// Infrastructure=SES for reputation + abuse correlation purposes.
	"_spf.resend.com":         {Name: "Resend", Category: CategoryESP, Infrastructure: InfraSES, Integration: IntegrationResendAPI},
	"_spf.loops.so":           {Name: "Loops", Category: CategoryESP, Infrastructure: InfraSES, Integration: IntegrationLoopsAPI},
	"_spf.bentonow.com":       {Name: "Bento", Category: CategoryESP, Infrastructure: InfraSES, Integration: IntegrationBentoAPI},
	"_spf.customeriomail.com": {Name: "Customer.io", Category: CategoryESP, Infrastructure: InfraSES, Integration: IntegrationCustomerIOAPI},
	"_spf.customer.io":        {Name: "Customer.io", Category: CategoryESP, Infrastructure: InfraSES, Integration: IntegrationCustomerIOAPI},
	"_spf.stripemail.com":     {Name: "Stripe", Category: CategoryESP, Infrastructure: InfraSES, Integration: IntegrationStripeManaged},

	// Salesforce transactional (Sales Cloud, Service Cloud, OWEA, Einstein
	// Activity Capture) — all SES-backed. Marketing Cloud (ExactTarget) is a
	// separate infra listed below.
	"_spf.salesforce.com":  {Name: "Salesforce", Category: CategoryESP, Infrastructure: InfraSES, Integration: IntegrationSalesforceOAuth},
	"_spfg.salesforce.com": {Name: "Salesforce", Category: CategoryESP, Infrastructure: InfraSES, Integration: IntegrationSalesforceOAuth},

	// Mailchimp / Pardot — partly SES. Flagged as ses_mixed because both SES
	// IPs and Mailchimp-owned IPs appear in their SPF expansion.
	"servers.mcsv.net": {Name: "Mailchimp", Category: CategoryESP, Infrastructure: InfraSESMixed, Integration: IntegrationMailchimpAPI},
	"_spf.mcsv.net":    {Name: "Mailchimp / Pardot", Category: CategoryESP, Infrastructure: InfraSESMixed, Integration: IntegrationMailchimpAPI},

	// Raw Amazon SES — customer is using SES directly with their own AWS
	// account. Needs IAM credential flow.
	"amazonses.com": {Name: "Amazon SES", Category: CategoryInfra, Infrastructure: InfraSES, Integration: IntegrationAWSIAM},

	// ---- Non-SES ESPs ----------------------------------------------------
	"sendgrid.net":    {Name: "SendGrid", Category: CategoryESP, Infrastructure: InfraTwilio, Integration: IntegrationSendGridAPI},
	"sendgrid.com":    {Name: "SendGrid", Category: CategoryESP, Infrastructure: InfraTwilio, Integration: IntegrationSendGridAPI},
	"spf.mtasv.net":   {Name: "Postmark", Category: CategoryESP, Infrastructure: InfraPostmark, Integration: IntegrationPostmarkAPI},
	"mailgun.org":     {Name: "Mailgun", Category: CategoryESP, Infrastructure: InfraMailgun, Integration: IntegrationMailgunAPI},
	"mg.mailgun.org":  {Name: "Mailgun", Category: CategoryESP, Infrastructure: InfraMailgun, Integration: IntegrationMailgunAPI},
	"spf.hubspot.com": {Name: "HubSpot", Category: CategoryESP, Infrastructure: InfraHubSpot, Integration: IntegrationHubSpotAPI},
	"sendinblue.com":  {Name: "Brevo (Sendinblue)", Category: CategoryESP, Infrastructure: InfraUnknown, Integration: IntegrationManual},
	"spf.brevo.com":   {Name: "Brevo", Category: CategoryESP, Infrastructure: InfraUnknown, Integration: IntegrationManual},

	// Salesforce Marketing Cloud (ExactTarget) — not SES, separate product.
	"cust-spf.exacttarget.com": {Name: "Salesforce Marketing Cloud", Category: CategoryESP, Infrastructure: InfraExactTarget, Integration: IntegrationManual},
	"_spf.exacttarget.com":     {Name: "Salesforce Marketing Cloud", Category: CategoryESP, Infrastructure: InfraExactTarget, Integration: IntegrationManual},

	// ---- Mailbox providers ----------------------------------------------
	"_spf.google.com":            {Name: "Google Workspace", Category: CategoryMailbox, Infrastructure: InfraGoogle, Integration: IntegrationGoogleOAuth},
	"spf.protection.outlook.com": {Name: "Microsoft 365", Category: CategoryMailbox, Infrastructure: InfraMicrosoft, Integration: IntegrationM365OAuth},
	"one.zoho.com":               {Name: "Zoho Mail", Category: CategoryMailbox, Infrastructure: InfraZoho, Integration: IntegrationZohoAPI},
	"_spf.zoho.com":              {Name: "Zoho Mail", Category: CategoryMailbox, Infrastructure: InfraZoho, Integration: IntegrationZohoAPI},
	"spf.messagingengine.com":    {Name: "Fastmail", Category: CategoryMailbox, Infrastructure: InfraFastmail, Integration: IntegrationManual},
	"icloud.com":                 {Name: "iCloud Mail", Category: CategoryMailbox, Infrastructure: InfraApple, Integration: IntegrationManual},
}

// DKIMTargetCatalog maps DKIM CNAME target suffixes (lowercased) to
// ESPInfo. Used when a domain's DKIM selector points at a known sender's
// signing infrastructure. Matched by suffix to handle per-tenant hostnames
// (e.g., "u1797798.wl049.sendgrid.net" matches "sendgrid.net").
var DKIMTargetCatalog = map[string]ESPInfo{
	"dkim.amazonses.com": {Name: "Amazon SES", Category: CategoryInfra, Infrastructure: InfraSES, Integration: IntegrationAWSIAM},
	"sendgrid.net":       {Name: "SendGrid", Category: CategoryESP, Infrastructure: InfraTwilio, Integration: IntegrationSendGridAPI},
	"resend.com":         {Name: "Resend", Category: CategoryESP, Infrastructure: InfraSES, Integration: IntegrationResendAPI},
	"pm.mtasv.net":       {Name: "Postmark", Category: CategoryESP, Infrastructure: InfraPostmark, Integration: IntegrationPostmarkAPI},
	"mailgun.org":        {Name: "Mailgun", Category: CategoryESP, Infrastructure: InfraMailgun, Integration: IntegrationMailgunAPI},
	"mcsv.net":           {Name: "Mailchimp", Category: CategoryESP, Infrastructure: InfraSESMixed, Integration: IntegrationMailchimpAPI},
	"hubspotemail.net":   {Name: "HubSpot", Category: CategoryESP, Infrastructure: InfraHubSpot, Integration: IntegrationHubSpotAPI},
	"salesforce.com":     {Name: "Salesforce", Category: CategoryESP, Infrastructure: InfraSES, Integration: IntegrationSalesforceOAuth},
	"gappssmtp.com":      {Name: "Google Workspace", Category: CategoryMailbox, Infrastructure: InfraGoogle, Integration: IntegrationGoogleOAuth},
	"outbound.protection.outlook.com": {Name: "Microsoft 365", Category: CategoryMailbox, Infrastructure: InfraMicrosoft, Integration: IntegrationM365OAuth},
	"zoho.com":           {Name: "Zoho Mail", Category: CategoryMailbox, Infrastructure: InfraZoho, Integration: IntegrationZohoAPI},
	"loops.so":           {Name: "Loops", Category: CategoryESP, Infrastructure: InfraSES, Integration: IntegrationLoopsAPI},
	"customeriomail.com": {Name: "Customer.io", Category: CategoryESP, Infrastructure: InfraSES, Integration: IntegrationCustomerIOAPI},
}

// LookupByInclude returns the ESPInfo matching an SPF include token.
// The include should be the value after "include:" (without the prefix).
// Returns ok=false if the include is not in the catalog.
func LookupByInclude(include string) (ESPInfo, bool) {
	info, ok := Catalog[strings.ToLower(strings.TrimSpace(include))]
	return info, ok
}

// LookupByDKIMTarget returns the ESPInfo matching a DKIM CNAME target host.
// Suffix matching: target "u1797798.wl049.sendgrid.net" matches catalog
// entry "sendgrid.net". Returns ok=false if no entry matches.
func LookupByDKIMTarget(target string) (ESPInfo, bool) {
	t := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(target), "."))
	if t == "" {
		return ESPInfo{}, false
	}
	// Try exact match first, then progressive suffix stripping.
	for host := t; host != ""; {
		if info, ok := DKIMTargetCatalog[host]; ok {
			return info, true
		}
		// Strip one label from the front and retry.
		idx := strings.Index(host, ".")
		if idx < 0 {
			break
		}
		host = host[idx+1:]
	}
	return ESPInfo{}, false
}
