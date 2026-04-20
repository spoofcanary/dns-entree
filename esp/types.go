// Package esp provides Email Service Provider detection and classification.
//
// Given a domain, esp inspects SPF includes and DKIM CNAME targets to
// identify which email senders are authorized and, critically, which
// underlying infrastructure those senders run on. A domain using Resend,
// Loops, Salesforce, or several other ESPs is effectively sending via
// Amazon SES - esp surfaces that so callers can make infrastructure-aware
// routing decisions (e.g., for DKIM key management, abuse correlation,
// reputation monitoring).
package esp

// Infrastructure identifies the underlying sending infrastructure a sender
// runs on. Some ESPs (Resend, Loops, Salesforce, Stripe transactional,
// Customer.io, parts of Mailchimp/Pardot) are layered on top of Amazon SES;
// InfraSES surfaces that even when the customer never touches AWS directly.
type Infrastructure string

const (
	InfraUnknown     Infrastructure = ""
	InfraSES         Infrastructure = "ses"         // Amazon SES (direct or via ESP)
	InfraSESMixed    Infrastructure = "ses_mixed"   // partly SES, partly other (e.g., Pardot)
	InfraGoogle      Infrastructure = "google"      // Google Workspace
	InfraMicrosoft   Infrastructure = "microsoft"   // M365 / Exchange Online
	InfraTwilio      Infrastructure = "twilio"      // SendGrid (Twilio-owned)
	InfraExactTarget Infrastructure = "exacttarget" // Salesforce Marketing Cloud
	InfraPostmark    Infrastructure = "postmark"
	InfraMailgun     Infrastructure = "mailgun"
	InfraMailchimp   Infrastructure = "mailchimp"
	InfraZoho        Infrastructure = "zoho"
	InfraFastmail    Infrastructure = "fastmail"
	InfraHubSpot     Infrastructure = "hubspot"
	InfraApple       Infrastructure = "apple"
)

// Category classifies a sender by the role it plays for the domain owner.
type Category string

const (
	CategoryUnknown Category = ""
	CategoryESP     Category = "esp"     // transactional/marketing email service
	CategoryMailbox Category = "mailbox" // mailbox provider (Google Workspace, M365, Zoho)
	CategoryInfra   Category = "infra"   // raw sending infrastructure (SES, self-hosted)
)

// Integration names the SendCanary integration path that unlocks automated
// DKIM / SPF / DMARC setup for a given sender. The customer-facing wizard
// uses this value to route to the correct credential prompt or OAuth flow.
//
// "manual" means no automation available - customer publishes records themselves.
type Integration string

const (
	IntegrationManual           Integration = "manual"
	IntegrationAWSIAM           Integration = "aws_iam"            // raw SES, needs AWS creds
	IntegrationResendAPI        Integration = "resend_api"
	IntegrationLoopsAPI         Integration = "loops_api"
	IntegrationBentoAPI         Integration = "bento_api"
	IntegrationCustomerIOAPI    Integration = "customerio_api"
	IntegrationSalesforceOAuth  Integration = "salesforce_oauth"
	IntegrationSendGridAPI      Integration = "sendgrid_api"
	IntegrationGoogleOAuth      Integration = "google_oauth"
	IntegrationM365OAuth        Integration = "m365_oauth"
	IntegrationPostmarkAPI      Integration = "postmark_api"
	IntegrationMailgunAPI       Integration = "mailgun_api"
	IntegrationMailchimpAPI     Integration = "mailchimp_api"
	IntegrationZohoAPI          Integration = "zoho_api"
	IntegrationStripeManaged    Integration = "stripe_managed" // no customer action - Stripe owns it
	IntegrationHubSpotAPI       Integration = "hubspot_api"
)

// ESPInfo describes a known sender and the infrastructure + integration path
// associated with it. ESPInfo values live in the catalog and are keyed by
// SPF include host or DKIM CNAME target suffix.
type ESPInfo struct {
	Name           string         // customer-visible display name
	Category       Category       // esp / mailbox / infra
	Infrastructure Infrastructure // underlying sending infra (may differ from Name)
	Integration    Integration    // how SendCanary automates setup
}

// SenderClassification is the result of classifying a single detected sender.
// Multiple classifications are typically returned per domain (e.g., Google
// Workspace for mailbox + Resend for transactional).
type SenderClassification struct {
	Name           string         `json:"name"`
	Category       Category       `json:"category"`
	Infrastructure Infrastructure `json:"infrastructure"`
	Integration    Integration    `json:"integration"`

	// SPFSource is the include: token that matched this sender. Empty if
	// classification came from DKIM alone.
	SPFSource string `json:"spf_source,omitempty"`

	// DKIMSelector is the selector name (before _domainkey) of a DKIM
	// record attributed to this sender. Empty if not found.
	DKIMSelector string `json:"dkim_selector,omitempty"`

	// DKIMTarget is the CNAME target for the DKIM record, when present.
	// Used to corroborate SPF-based classification.
	DKIMTarget string `json:"dkim_target,omitempty"`

	// ViaChain is true when Infrastructure was derived by walking the SPF
	// include chain (e.g., _spf.resend.com -> amazonses.com). False means
	// the top-level include matched directly.
	ViaChain bool `json:"via_chain,omitempty"`
}
