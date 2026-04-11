package domainconnect

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func publicResolver(_ context.Context, _ string) ([]net.IP, error) {
	return []net.IP{net.ParseIP("8.8.8.8")}, nil
}

func txtFor(value string) func(context.Context, string) ([]string, error) {
	return func(_ context.Context, _ string) ([]string, error) {
		return []string{value}, nil
	}
}

// hostFromTestServer returns the host:port from an httptest server URL.
func hostFromTestServer(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return u.Host
}

func newDiscoverClient(srv *httptest.Server, o *discoverOptions) *http.Client {
	c := srv.Client()
	c.CheckRedirect = makeCheckRedirect(o)
	return c
}

func TestDiscover_Success(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/example.com/settings" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"providerId":"p","providerName":"P","urlSyncUX":"https://s","urlAsyncUX":"https://a","urlAPI":"https://api","width":600,"height":750}`)
	}))
	defer srv.Close()

	o := &discoverOptions{ipResolver: publicResolver}
	res, err := Discover(context.Background(), "example.com",
		WithTXTResolver(txtFor(hostFromTestServer(t, srv))),
		WithResolver(publicResolver),
		WithHTTPClient(newDiscoverClient(srv, o)),
	)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !res.Supported {
		t.Fatal("expected Supported=true")
	}
	if res.ProviderID != "p" || res.ProviderName != "P" || res.URLSyncUX != "https://s" ||
		res.URLAsyncUX != "https://a" || res.URLAPI != "https://api" ||
		res.Width != 600 || res.Height != 750 {
		t.Errorf("fields wrong: %+v", res)
	}
	if len(res.Nameservers) != 1 || res.Nameservers[0] != "8.8.8.8" {
		t.Errorf("nameservers wrong: %+v", res.Nameservers)
	}
}

func TestDiscover_NoTXT(t *testing.T) {
	res, err := Discover(context.Background(), "example.com",
		WithTXTResolver(func(context.Context, string) ([]string, error) { return nil, nil }),
	)
	if err != nil || res.Supported {
		t.Fatalf("got %+v err=%v", res, err)
	}
}

func TestDiscover_TXTError(t *testing.T) {
	res, err := Discover(context.Background(), "example.com",
		WithTXTResolver(func(context.Context, string) ([]string, error) {
			return nil, fmt.Errorf("nx")
		}),
	)
	if err != nil || res.Supported {
		t.Fatalf("got %+v err=%v", res, err)
	}
}

func ssrfCase(t *testing.T, ips ...net.IP) {
	t.Helper()
	res, err := Discover(context.Background(), "example.com",
		WithTXTResolver(txtFor("attacker.example")),
		WithResolver(func(context.Context, string) ([]net.IP, error) { return ips, nil }),
	)
	if err != nil || res.Supported {
		t.Fatalf("expected blocked, got %+v err=%v", res, err)
	}
}

func TestDiscover_SSRF_Loopback(t *testing.T)        { ssrfCase(t, net.ParseIP("127.0.0.1")) }
func TestDiscover_SSRF_Private(t *testing.T)         { ssrfCase(t, net.ParseIP("10.0.0.1")) }
func TestDiscover_SSRF_LinkLocal(t *testing.T)       { ssrfCase(t, net.ParseIP("169.254.169.254")) }
func TestDiscover_SSRF_IPv6Unspecified(t *testing.T) { ssrfCase(t, net.ParseIP("::")) }
func TestDiscover_SSRF_Multicast(t *testing.T)       { ssrfCase(t, net.ParseIP("224.0.0.1")) }
func TestDiscover_SSRF_MixedIPs(t *testing.T) {
	ssrfCase(t, net.ParseIP("8.8.8.8"), net.ParseIP("10.0.0.1"))
}

func TestDiscover_SSRF_ResolverError(t *testing.T) {
	res, err := Discover(context.Background(), "example.com",
		WithTXTResolver(txtFor("attacker.example")),
		WithResolver(func(context.Context, string) ([]net.IP, error) {
			return nil, fmt.Errorf("nope")
		}),
	)
	if err != nil || res.Supported {
		t.Fatalf("got %+v err=%v", res, err)
	}
}

func TestDiscover_HTTPNon200(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	o := &discoverOptions{ipResolver: publicResolver}
	res, err := Discover(context.Background(), "example.com",
		WithTXTResolver(txtFor(hostFromTestServer(t, srv))),
		WithResolver(publicResolver),
		WithHTTPClient(newDiscoverClient(srv, o)),
	)
	if err != nil || res.Supported {
		t.Fatalf("got %+v err=%v", res, err)
	}
}

func TestDiscover_BadJSON(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "not json")
	}))
	defer srv.Close()
	o := &discoverOptions{ipResolver: publicResolver}
	res, err := Discover(context.Background(), "example.com",
		WithTXTResolver(txtFor(hostFromTestServer(t, srv))),
		WithResolver(publicResolver),
		WithHTTPClient(newDiscoverClient(srv, o)),
	)
	if err != nil || res.Supported {
		t.Fatalf("got %+v err=%v", res, err)
	}
}

func TestDiscover_BodyTooLarge(t *testing.T) {
	chunk := strings.Repeat(" ", 1024)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// 200KB of leading whitespace then truncated JSON. LimitReader at 64KB
		// will cut us off mid-whitespace; decode must fail.
		for i := 0; i < 200; i++ {
			_, _ = w.Write([]byte(chunk))
		}
		fmt.Fprint(w, `{"providerId":"p"}`)
	}))
	defer srv.Close()
	o := &discoverOptions{ipResolver: publicResolver}
	res, err := Discover(context.Background(), "example.com",
		WithTXTResolver(txtFor(hostFromTestServer(t, srv))),
		WithResolver(publicResolver),
		WithHTTPClient(newDiscoverClient(srv, o)),
	)
	if err != nil || res.Supported {
		t.Fatalf("got %+v err=%v", res, err)
	}
}

func TestDiscover_RedirectToInternal(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Redirect(w, &http.Request{}, "http://169.254.169.254/x", http.StatusFound)
	}))
	defer srv.Close()
	o := &discoverOptions{ipResolver: publicResolver}
	res, err := Discover(context.Background(), "example.com",
		WithTXTResolver(txtFor(hostFromTestServer(t, srv))),
		WithResolver(publicResolver),
		WithHTTPClient(newDiscoverClient(srv, o)),
	)
	if err != nil || res.Supported {
		t.Fatalf("got %+v err=%v", res, err)
	}
}

func TestDiscover_RedirectToHTTP(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Redirect(w, &http.Request{}, "http://other.example/x", http.StatusFound)
	}))
	defer srv.Close()
	o := &discoverOptions{ipResolver: publicResolver}
	res, err := Discover(context.Background(), "example.com",
		WithTXTResolver(txtFor(hostFromTestServer(t, srv))),
		WithResolver(publicResolver),
		WithHTTPClient(newDiscoverClient(srv, o)),
	)
	if err != nil || res.Supported {
		t.Fatalf("got %+v err=%v", res, err)
	}
}

func TestDiscover_EmptyDomain(t *testing.T) {
	_, err := Discover(context.Background(), "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDiscover_TXTPathStripped(t *testing.T) {
	var seen string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/path/v2/example.com/settings" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{"providerId":"p","providerName":"P"}`)
	}))
	defer srv.Close()
	o := &discoverOptions{ipResolver: publicResolver}
	txt := hostFromTestServer(t, srv) + "/path"
	res, err := Discover(context.Background(), "example.com",
		WithTXTResolver(txtFor(txt)),
		WithResolver(func(ctx context.Context, host string) ([]net.IP, error) {
			seen = host
			return []net.IP{net.ParseIP("8.8.8.8")}, nil
		}),
		WithHTTPClient(newDiscoverClient(srv, o)),
	)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !res.Supported {
		t.Fatal("expected supported")
	}
	if strings.Contains(seen, "/") {
		t.Errorf("ip resolver received host with path: %q", seen)
	}
	if !strings.HasPrefix(seen, strings.SplitN(hostFromTestServer(t, srv), ":", 2)[0]) &&
		seen != hostFromTestServer(t, srv) {
		t.Errorf("unexpected host: %q", seen)
	}
}
