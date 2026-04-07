package migrate

// DefaultLabels is the well-known label list iterated scrape walks by default.
// See decision D-07.
var DefaultLabels = []string{
	"@", "*", "www", "mail", "webmail", "autodiscover", "autoconfig",
	"ftp", "sftp", "vpn",
	"ns1", "ns2", "ns3", "ns4",
	"api", "app", "admin", "dev", "staging", "beta", "blog", "shop",
	"cdn", "static", "assets", "files", "docs", "status", "support", "help",
	"_dmarc", "_domainkey", "_mta-sts", "_smtp._tls", "_acme-challenge",
	"selector1._domainkey", "selector2._domainkey", "google._domainkey",
	"k1._domainkey", "k2._domainkey", "s1._domainkey", "s2._domainkey",
	"default._domainkey", "smtp._domainkey", "mandrill._domainkey",
	"_github-challenge", "_globalsign-domain-verification", "_atproto",
	"_spf", "_psl", "_domainconnect",
}
