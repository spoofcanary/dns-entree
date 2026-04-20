package esp

import "testing"

func TestLookupByInclude(t *testing.T) {
	cases := []struct {
		include    string
		wantName   string
		wantInfra  Infrastructure
		wantOK     bool
	}{
		{"amazonses.com", "Amazon SES", InfraSES, true},
		{"_spf.resend.com", "Resend", InfraSES, true},
		{"_spf.salesforce.com", "Salesforce", InfraSES, true},
		{"sendgrid.net", "SendGrid", InfraTwilio, true},
		{"_spf.google.com", "Google Workspace", InfraGoogle, true},
		{"cust-spf.exacttarget.com", "Salesforce Marketing Cloud", InfraExactTarget, true},
		{"AMAZONSES.COM", "Amazon SES", InfraSES, true}, // case-insensitive
		{"unknown.example.com", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.include, func(t *testing.T) {
			info, ok := LookupByInclude(tc.include)
			if ok != tc.wantOK {
				t.Fatalf("ok=%v want %v", ok, tc.wantOK)
			}
			if ok {
				if info.Name != tc.wantName {
					t.Errorf("name=%q want %q", info.Name, tc.wantName)
				}
				if info.Infrastructure != tc.wantInfra {
					t.Errorf("infra=%q want %q", info.Infrastructure, tc.wantInfra)
				}
			}
		})
	}
}

func TestLookupByDKIMTarget_SuffixMatch(t *testing.T) {
	cases := []struct {
		target    string
		wantName  string
		wantInfra Infrastructure
		wantOK    bool
	}{
		{"u1797798.wl049.sendgrid.net", "SendGrid", InfraTwilio, true},
		{"abc123.dkim.amazonses.com", "Amazon SES", InfraSES, true},
		{"abc123.dkim.amazonses.com.", "Amazon SES", InfraSES, true}, // trailing dot
		{"s1.domainkey.example.resend.com", "Resend", InfraSES, true},
		{"sig1._domainkey.loops.so", "Loops", InfraSES, true},
		{"mydomain.gappssmtp.com", "Google Workspace", InfraGoogle, true},
		{"example.com.outbound.protection.outlook.com", "Microsoft 365", InfraMicrosoft, true},
		{"", "", "", false},
		{"unrelated.example.com", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.target, func(t *testing.T) {
			info, ok := LookupByDKIMTarget(tc.target)
			if ok != tc.wantOK {
				t.Fatalf("ok=%v want %v", ok, tc.wantOK)
			}
			if ok {
				if info.Name != tc.wantName {
					t.Errorf("name=%q want %q", info.Name, tc.wantName)
				}
				if info.Infrastructure != tc.wantInfra {
					t.Errorf("infra=%q want %q", info.Infrastructure, tc.wantInfra)
				}
			}
		})
	}
}
