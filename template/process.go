package template

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	entree "github.com/spoofcanary/dns-entree"
)

// ProcessOpts configures a ProcessRecords call.
type ProcessOpts struct {
	Domain          string
	Host            string           // subdomain or empty for apex
	ZoneRecords     []entree.Record  // existing zone state
	TemplateRecords []TemplateRecord // from the template JSON
	Variables       map[string]string
	GroupIDs        []string // optional group filter; empty = all groups
	MultiAware      bool
	MultiInstance   bool
	ProviderID      string
	ServiceID       string
	UniqueID        string // for multi-instance; random if empty + MultiAware
	RedirectRecords []TemplateRecord
	IgnoreSignature bool

	// Signature verification fields.
	Signature        string           // base64-encoded RSA signature
	SigningKey       string           // key= parameter value (DNS host prefix)
	QueryString      string           // the signed query string
	SyncPubKeyDomain string           // from template JSON
	PubKeyLookup     PubKeyLookupFunc // DNS TXT lookup; nil = use net.LookupTXT
}

// ProcessResult holds the output of ProcessRecords.
type ProcessResult struct {
	ToAdd    []entree.Record
	ToDelete []entree.Record
}

// DC error types for compliance mapping.
type InvalidDataError struct{ Msg string }
type MissingParameterError struct{ Msg string }
type InvalidTemplateError struct{ Msg string }
type TypeErrorError struct{ Msg string }

func (e *InvalidDataError) Error() string      { return "InvalidData: " + e.Msg }
func (e *MissingParameterError) Error() string { return "MissingParameter: " + e.Msg }
func (e *InvalidTemplateError) Error() string  { return "InvalidTemplate: " + e.Msg }
func (e *TypeErrorError) Error() string        { return "TypeError: " + e.Msg }

// Known DNS record types (core + common custom).
var knownTypes = map[string]bool{
	"A": true, "AAAA": true, "CNAME": true, "TXT": true,
	"MX": true, "NS": true, "SRV": true, "SPFM": true,
	"REDIR301": true, "REDIR302": true, "APEXCNAME": true,
	"CAA": true,
}

// isCustomType returns true for CAA, TYPE<N>, and other non-core types.
func isCustomType(typ string) bool {
	switch typ {
	case "A", "AAAA", "CNAME", "TXT", "MX", "NS", "SRV", "SPFM",
		"REDIR301", "REDIR302", "APEXCNAME":
		return false
	}
	return true
}

// validRecordType checks if a type string is valid.
func validRecordType(typ string) bool {
	if knownTypes[typ] {
		return true
	}
	// TYPE<N> RFC 3597
	if strings.HasPrefix(typ, "TYPE") && len(typ) > 4 {
		for _, c := range typ[4:] {
			if c < '0' || c > '9' {
				return false
			}
		}
		return true
	}
	// CAA and other alpha-only types
	for _, c := range typ {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return len(typ) > 0
}

// validProtocol checks SRV protocol.
var validProtocols = map[string]bool{
	"udp": true, "tcp": true, "tls": true,
	"UDP": true, "TCP": true, "TLS": true,
}

// processVarRegex matches %var% tokens for unclosed-var detection.
var processVarRegex = regexp.MustCompile(`%([a-zA-Z0-9_-]+)%`)

// ProcessRecords implements the DC spec's process_records zone-apply algorithm.
func ProcessRecords(opts ProcessOpts) (*ProcessResult, error) {
	// Signature verification (before any zone mutations).
	if opts.Signature != "" && !opts.IgnoreSignature {
		lookup := opts.PubKeyLookup
		if lookup == nil {
			lookup = defaultPubKeyLookup
		}
		if err := VerifySignature(opts.QueryString, opts.Signature, opts.SigningKey, opts.SyncPubKeyDomain, lookup); err != nil {
			return nil, err
		}
	}

	domain := strings.ToLower(opts.Domain)
	host := opts.Host

	// Build variable map with implicit vars.
	vars := make(map[string]string)
	for k, v := range opts.Variables {
		vars[k] = v
	}
	vars["domain"] = domain
	vars["host"] = host
	if host != "" && host != "@" {
		vars["fqdn"] = host + "." + domain
	} else {
		vars["fqdn"] = domain
	}

	// Filter by group IDs if specified.
	templateRecords := opts.TemplateRecords
	if len(opts.GroupIDs) > 0 {
		groupSet := make(map[string]bool)
		for _, g := range opts.GroupIDs {
			groupSet[g] = true
		}
		var filtered []TemplateRecord
		for _, tr := range templateRecords {
			if tr.GroupID != "" && groupSet[tr.GroupID] {
				filtered = append(filtered, tr)
			}
		}
		templateRecords = filtered
	}

	// Resolve all template records into concrete records.
	var resolved []resolvedEntry

	for i, tr := range templateRecords {
		typ := strings.ToUpper(strings.TrimSpace(tr.Type))

		// Validate record type.
		if !validRecordType(typ) {
			return nil, &TypeErrorError{Msg: fmt.Sprintf("record %d: invalid type %q", i, tr.Type)}
		}

		// Check for unclosed variables in all string fields.
		for _, field := range []string{tr.Host, tr.PointsTo, tr.Target, tr.Data, tr.TxtConflictMatchingPrefix} {
			if err := checkUnclosedVar(field); err != nil {
				return nil, &InvalidTemplateError{Msg: fmt.Sprintf("record %d: %v", i, err)}
			}
		}

		// Variable substitution.
		recHost := tr.Host
		if recHost == "" && tr.Type == "SRV" {
			// SRV uses "name" field, already mapped to Host by buildTemplate.
		}

		hostSub, err := processSubstitute(recHost, vars, i, "host")
		if err != nil {
			return nil, err
		}

		pointsTo := tr.PointsTo
		if pointsTo == "" {
			pointsTo = tr.Target
		}
		pointsToSub, err := processSubstitute(pointsTo, vars, i, "pointsTo")
		if err != nil {
			return nil, err
		}

		dataSub, err := processSubstitute(tr.Data, vars, i, "data")
		if err != nil {
			return nil, err
		}

		prefixSub, err := processSubstitute(tr.TxtConflictMatchingPrefix, vars, i, "txtConflictMatchingPrefix")
		if err != nil {
			return nil, err
		}

		spfRules := tr.Data
		if spfRules == "" && tr.PointsTo != "" {
			spfRules = tr.PointsTo
		}
		spfRulesSub, err := processSubstitute(spfRules, vars, i, "spfRules")
		if err != nil {
			return nil, err
		}

		// Resolve host: apply @ resolution + host appending + FQDN trimming.
		resolvedHost, err := resolveHostName(hostSub, host, domain)
		if err != nil {
			return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d host: %v", i, err)}
		}

		// Validate FQDN length: host + "." + domain must be <= 253.
		if resolvedHost != "" && resolvedHost != "@" {
			fqdnLen := len(resolvedHost) + 1 + len(domain)
			if fqdnLen > 253 {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d host: FQDN %s.%s exceeds 253 characters (%d)", i, resolvedHost, domain, fqdnLen)}
			}
		}

		// Resolve pointsTo: @ -> fqdn.
		resolvedPointsTo := resolvePointsTo(pointsToSub, host, domain, typ)

		// Handle SRV protocol normalization.
		protocol := strings.TrimPrefix(tr.Protocol, "_")
		protocol = strings.ToLower(protocol)

		// Resolve TTL, priority, weight, port.
		ttl, err := tr.TTL.resolve(vars, i, "ttl")
		if err != nil {
			return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d: %v", i, err)}
		}

		// Build the record based on type.
		var rec entree.Record
		switch typ {
		case "REDIR301", "REDIR302":
			// Validate redirect records are present.
			if len(opts.RedirectRecords) == 0 {
				return nil, &InvalidTemplateError{Msg: fmt.Sprintf("record %d: %s requires redirect_records", i, typ)}
			}
			// Resolve target URL.
			targetURL, err := processSubstitute(pointsTo, vars, i, "target")
			if err != nil {
				return nil, err
			}
			if targetURL == "" {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d: %s target is empty", i, typ)}
			}
			// Validate URL.
			if _, uerr := url.Parse(targetURL); uerr != nil || !isValidRedirectURL(targetURL) {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d: %s invalid target URL %q", i, typ, targetURL)}
			}

			rec = entree.Record{
				Type:    typ,
				Name:    resolvedHost,
				Content: targetURL,
				TTL:     0, // REDIR records don't have a meaningful TTL in the zone
			}
			resolved = append(resolved, resolvedEntry{rec: rec, tmplRec: tr, origType: typ})

			// Add backing A/AAAA records at the same host.
			for _, rr := range opts.RedirectRecords {
				backingType := strings.ToUpper(rr.Type)
				backingPointsTo := rr.PointsTo
				if backingPointsTo == "" {
					backingPointsTo = rr.Target
				}
				backingTTL, _ := rr.TTL.resolve(vars, 0, "ttl")
				backingRec := normalizeRecord(entree.Record{
					Type:    backingType,
					Name:    resolvedHost,
					Content: backingPointsTo,
					TTL:     backingTTL,
				})
				resolved = append(resolved, resolvedEntry{
					rec:       backingRec,
					tmplRec:   rr,
					origType:  backingType,
					isBacking: true,
				})
			}
			continue

		case "APEXCNAME":
			// APEXCNAME must be at apex.
			if resolvedHost != "@" && resolvedHost != "" {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d: APEXCNAME must be at apex, got host %q", i, resolvedHost)}
			}
			resolvedHost = "@"
			if resolvedPointsTo == "" {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d: APEXCNAME pointsTo is empty", i)}
			}
			rec = entree.Record{
				Type:    "APEXCNAME",
				Name:    resolvedHost,
				Content: resolvedPointsTo,
				TTL:     ttl,
			}

		case "SPFM":
			// Validate host.
			if err := processValidateHost(resolvedHost); err != nil {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d host: %v", i, err)}
			}
			// SPFM merges into existing SPF. Handle later in zone conflict phase.
			rec = entree.Record{
				Type:    "SPFM",
				Name:    resolvedHost,
				Content: spfRulesSub,
				TTL:     ttl,
			}
			resolved = append(resolved, resolvedEntry{rec: rec, tmplRec: tr, origType: "SPFM"})
			continue

		case "SRV":
			// Validate host.
			if err := processValidateHost(resolvedHost); err != nil {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d host: %v", i, err)}
			}
			// SRV @ at apex (no host) is invalid.
			if (resolvedHost == "@" || resolvedHost == "") && host == "" {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d: SRV at apex is invalid", i)}
			}
			// Validate protocol.
			if protocol != "" && !validProtocols[protocol] {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d: invalid SRV protocol %q", i, tr.Protocol)}
			}
			// Validate target.
			if resolvedPointsTo == "" {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d: SRV target is empty", i)}
			}
			if strings.ContainsAny(resolvedPointsTo, " \t") {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d: SRV target contains spaces", i)}
			}
			// Validate service.
			if strings.ContainsAny(tr.Service, " \t") {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d: SRV service contains spaces", i)}
			}

			prio, err := tr.Priority.resolve(vars, i, "priority")
			if err != nil {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d: %v", i, err)}
			}
			w, err := tr.Weight.resolve(vars, i, "weight")
			if err != nil {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d: %v", i, err)}
			}
			p, err := tr.Port.resolve(vars, i, "port")
			if err != nil {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d: %v", i, err)}
			}
			rec = entree.Record{
				Type:     "SRV",
				Name:     resolvedHost,
				Content:  resolvedPointsTo,
				TTL:      ttl,
				Priority: prio,
				Weight:   w,
				Port:     p,
				Service:  tr.Service,
				Protocol: protocol,
			}

		case "TXT":
			if err := processValidateHost(resolvedHost); err != nil {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d host: %v", i, err)}
			}
			rec = entree.Record{
				Type:    "TXT",
				Name:    resolvedHost,
				Content: dataSub,
				TTL:     ttl,
			}

		case "CNAME":
			if err := processValidateHost(resolvedHost); err != nil {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d host: %v", i, err)}
			}
			// CNAME at apex is invalid.
			if resolvedHost == "@" || resolvedHost == "" {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d: CNAME at apex is invalid", i)}
			}
			if resolvedPointsTo == "" {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d: CNAME pointsTo is empty", i)}
			}
			if err := validatePointsToLength(resolvedPointsTo); err != nil {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d pointsTo: %v", i, err)}
			}
			rec = entree.Record{
				Type:    "CNAME",
				Name:    resolvedHost,
				Content: resolvedPointsTo,
				TTL:     ttl,
			}

		case "A", "AAAA":
			if err := processValidateHost(resolvedHost); err != nil {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d host: %v", i, err)}
			}
			if resolvedPointsTo == "" {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d: %s pointsTo is empty", i, typ)}
			}
			if err := processValidateIP(typ, resolvedPointsTo); err != nil {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d pointsTo: %v", i, err)}
			}
			rec = entree.Record{
				Type:    typ,
				Name:    resolvedHost,
				Content: resolvedPointsTo,
				TTL:     ttl,
			}

		case "MX":
			if err := processValidateHost(resolvedHost); err != nil {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d host: %v", i, err)}
			}
			if resolvedPointsTo == "" {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d: MX pointsTo is empty", i)}
			}
			prio, err := tr.Priority.resolve(vars, i, "priority")
			if err != nil {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d: %v", i, err)}
			}
			rec = entree.Record{
				Type:     "MX",
				Name:     resolvedHost,
				Content:  resolvedPointsTo,
				TTL:      ttl,
				Priority: prio,
			}

		case "NS":
			if err := processValidateHost(resolvedHost); err != nil {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d host: %v", i, err)}
			}
			if resolvedPointsTo == "" {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d: NS pointsTo is empty", i)}
			}
			// NS @ resolves to fqdn, which for pointsTo means self-delegation -> error.
			if pointsToSub == "@" {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d: NS pointsTo cannot be self (circular)", i)}
			}
			rec = entree.Record{
				Type:    "NS",
				Name:    resolvedHost,
				Content: resolvedPointsTo,
				TTL:     ttl,
			}

		default:
			// Custom types (CAA, TYPE<N>, etc.)
			if err := processValidateHost(resolvedHost); err != nil {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d host: %v", i, err)}
			}
			// @ in data for custom types is invalid.
			if dataSub == "@" {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d: @ in custom record data is invalid", i)}
			}
			if dataSub == "" {
				return nil, &InvalidDataError{Msg: fmt.Sprintf("record %d: custom record data is empty", i)}
			}
			rec = entree.Record{
				Type:    typ,
				Name:    resolvedHost,
				Content: dataSub,
				TTL:     ttl,
			}
		}

		_ = prefixSub
		rec = normalizeRecord(rec)
		resolved = append(resolved, resolvedEntry{rec: rec, tmplRec: tr, origType: typ})
	}

	// Check for conflicts between generated records.
	if err := checkInternalConflicts(resolved); err != nil {
		return nil, err
	}

	// Check for multiple APEXCNAME.
	apexCnameCount := 0
	for _, re := range resolved {
		if re.rec.Type == "APEXCNAME" {
			apexCnameCount++
		}
	}
	if apexCnameCount > 1 {
		return nil, &InvalidDataError{Msg: "multiple APEXCNAME records in template"}
	}

	// Build zone state as a mutable slice, normalizing records.
	zone := make([]entree.Record, len(opts.ZoneRecords))
	for i, r := range opts.ZoneRecords {
		zone[i] = normalizeRecord(r)
	}

	// Multi-aware unique ID generation.
	uniqueID := opts.UniqueID
	if opts.MultiAware && uniqueID == "" {
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		uniqueID = hex.EncodeToString(b)
	}

	var toAdd []entree.Record
	var toDelete []entree.Record

	// Multi-aware pre-deletion: remove records owned by the same provider
	// before applying the new template.
	var essentialAlwaysCandidates []entree.Record // may be restored later
	if opts.MultiAware {
		var multiDels []entree.Record
		for _, zr := range zone {
			if zr.DC == nil {
				continue
			}
			zrProvider, _ := zr.DC["providerId"].(string)
			zrService, _ := zr.DC["serviceId"].(string)
			zrID, _ := zr.DC["id"].(string)
			zrEssential, _ := zr.DC["essential"].(string)

			sameService := zrProvider == opts.ProviderID && zrService == opts.ServiceID

			if opts.MultiInstance {
				if sameService && zrID == uniqueID {
					multiDels = append(multiDels, zr)
				}
			} else if sameService {
				// Same service re-apply: replace all.
				multiDels = append(multiDels, zr)
			} else if zrProvider == opts.ProviderID {
				// Different service, same provider.
				if zrEssential == "Always" {
					// Tentatively delete; may restore if no conflict.
					essentialAlwaysCandidates = append(essentialAlwaysCandidates, zr)
				}
				multiDels = append(multiDels, zr)
			}
		}
		toDelete = append(toDelete, multiDels...)
		zone = removeRecords(zone, multiDels)
	}

	// Track original zone records so SRV conflict resolution only targets
	// pre-existing records, not records added by this template batch.
	originalZone := make(map[string]int)
	for _, r := range zone {
		originalZone[recordKey(r)]++
	}

	// Process each resolved record against the zone.
	for _, re := range resolved {
		rec := re.rec
		typ := rec.Type
		name := rec.Name

		// Backing records (from REDIR) are added directly without conflict rules.
		if re.isBacking {
			toAdd = append(toAdd, rec)
			zone = append(zone, rec)
			continue
		}

		switch typ {
		case "SPFM":
			// SPFM merge logic.
			spfAdd, spfDel := processSPFM(zone, rec)
			toAdd = append(toAdd, spfAdd...)
			toDelete = append(toDelete, spfDel...)
			// Update zone.
			zone = removeRecords(zone, spfDel)
			zone = append(zone, spfAdd...)

		case "CNAME":
			// CNAME at a name deletes all non-NS records at that name.
			var dels []entree.Record
			for _, zr := range zone {
				if nameEq(zr.Name, name) && zr.Type != "NS" {
					dels = append(dels, zr)
				}
			}
			toDelete = append(toDelete, dels...)
			zone = removeRecords(zone, dels)
			toAdd = append(toAdd, rec)
			zone = append(zone, rec)

		case "A", "AAAA":
			// A/AAAA at a name deletes existing A, AAAA, CNAME at that name.
			var dels []entree.Record
			for _, zr := range zone {
				if nameEq(zr.Name, name) && (zr.Type == "A" || zr.Type == "AAAA" || zr.Type == "CNAME") {
					dels = append(dels, zr)
				}
			}
			toDelete = append(toDelete, dels...)
			zone = removeRecords(zone, dels)
			// Also delete NS at same name (A/AAAA replaces NS delegation at same level).
			var nsDels []entree.Record
			for _, zr := range zone {
				if nameEq(zr.Name, name) && zr.Type == "NS" {
					nsDels = append(nsDels, zr)
				}
			}
			toDelete = append(toDelete, nsDels...)
			zone = removeRecords(zone, nsDels)
			toAdd = append(toAdd, rec)
			zone = append(zone, rec)

		case "TXT":
			// TXT conflict modes.
			mode := re.tmplRec.TxtConflictMatchingMode
			prefix, _ := processSubstitute(re.tmplRec.TxtConflictMatchingPrefix, vars, 0, "prefix")
			var dels []entree.Record
			if mode == "" {
				// Default: delete CNAME at same name only.
				for _, zr := range zone {
					if nameEq(zr.Name, name) && zr.Type == "CNAME" {
						dels = append(dels, zr)
					}
				}
			} else {
				switch mode {
				case "None":
					// No deletes.
				case "All":
					for _, zr := range zone {
						if nameEq(zr.Name, name) && zr.Type == "TXT" {
							dels = append(dels, zr)
						}
					}
				case "Prefix":
					for _, zr := range zone {
						if nameEq(zr.Name, name) && zr.Type == "TXT" && strings.HasPrefix(zr.Content, prefix) {
							dels = append(dels, zr)
						}
					}
				}
			}
			toDelete = append(toDelete, dels...)
			zone = removeRecords(zone, dels)
			// Dedup: if identical TXT (same name+data) already in zone, skip.
			alreadyExists := false
			for _, zr := range zone {
				if zr.Type == "TXT" && nameEq(zr.Name, name) && zr.Content == rec.Content {
					alreadyExists = true
					break
				}
			}
			if !alreadyExists {
				toAdd = append(toAdd, rec)
				zone = append(zone, rec)
			}

		case "MX":
			// MX replaces existing MX at same name.
			var dels []entree.Record
			for _, zr := range zone {
				if nameEq(zr.Name, name) && zr.Type == "MX" {
					dels = append(dels, zr)
				}
			}
			toDelete = append(toDelete, dels...)
			zone = removeRecords(zone, dels)
			toAdd = append(toAdd, rec)
			zone = append(zone, rec)

		case "NS":
			// NS at a name: delete all records at and below this name (cascade),
			// including existing NS at the exact same name from the original zone.
			var dels []entree.Record
			for _, zr := range zone {
				if zr.Type == "NS" {
					// Only delete NS at exact same name from the original zone.
					if nameEq(zr.Name, name) {
						k := recordKey(zr)
						if originalZone[k] > 0 {
							dels = append(dels, zr)
						}
					}
					continue
				}
				if nameEq(zr.Name, name) || isChildOf(zr.Name, name) {
					dels = append(dels, zr)
				}
			}
			toDelete = append(toDelete, dels...)
			zone = removeRecords(zone, dels)
			toAdd = append(toAdd, rec)
			zone = append(zone, rec)

		case "SRV":
			// SRV replaces existing SRV at same name, but only from the
			// original zone (not records added by this template batch).
			var dels []entree.Record
			for _, zr := range zone {
				if nameEq(zr.Name, name) && zr.Type == "SRV" {
					k := recordKey(zr)
					if originalZone[k] > 0 {
						dels = append(dels, zr)
					}
				}
			}
			toDelete = append(toDelete, dels...)
			zone = removeRecords(zone, dels)
			toAdd = append(toAdd, rec)
			zone = append(zone, rec)

		case "REDIR301", "REDIR302":
			// REDIR replaces A/AAAA/CNAME at the same host.
			var dels []entree.Record
			for _, zr := range zone {
				if nameEq(zr.Name, name) && (zr.Type == "A" || zr.Type == "AAAA" || zr.Type == "CNAME" || zr.Type == "REDIR301" || zr.Type == "REDIR302") {
					dels = append(dels, zr)
				}
			}
			toDelete = append(toDelete, dels...)
			zone = removeRecords(zone, dels)
			toAdd = append(toAdd, rec)
			zone = append(zone, rec)

		case "APEXCNAME":
			// Delete conflicting records at @ (A, AAAA, CNAME, MX, TXT) but not NS.
			var dels []entree.Record
			for _, zr := range zone {
				if nameEq(zr.Name, "@") && zr.Type != "NS" {
					dels = append(dels, zr)
				}
			}
			toDelete = append(toDelete, dels...)
			zone = removeRecords(zone, dels)
			toAdd = append(toAdd, rec)
			zone = append(zone, rec)

		default:
			// Custom types: no conflict deletion. Dedup against zone.
			alreadyExists := false
			for _, zr := range zone {
				if zr.Type == rec.Type && nameEq(zr.Name, name) && zr.Content == rec.Content {
					alreadyExists = true
					break
				}
			}
			if !alreadyExists {
				toAdd = append(toAdd, rec)
				zone = append(zone, rec)
			}
		}
	}

	// Restore essential=Always records from different-service deletion if they
	// don't conflict with any newly added record (same type+name).
	// However, if ANY essential=Always record from the same old instance
	// directly conflicts with a new record, ALL records from that instance
	// are cascade-deleted (no restoration).
	if len(essentialAlwaysCandidates) > 0 {
		// Check if any essential=Always candidate conflicts with new records.
		anyConflict := false
		for _, ea := range essentialAlwaysCandidates {
			for _, added := range toAdd {
				if strings.ToUpper(added.Type) == strings.ToUpper(ea.Type) && nameEq(added.Name, ea.Name) {
					anyConflict = true
					break
				}
			}
			if anyConflict {
				break
			}
		}
		// Only restore if no essential=Always record conflicted.
		if !anyConflict {
			for _, ea := range essentialAlwaysCandidates {
				toDelete = removeRecords(toDelete, []entree.Record{ea})
				zone = append(zone, ea)
			}
		}
	}

	// Deduplicate toAdd.
	toAdd = deduplicateRecords(toAdd)

	// Deduplicate toDelete.
	toDelete = deduplicateRecords(toDelete)

	// Stamp _dc metadata on added records when multi-aware.
	if opts.MultiAware {
		for i := range toAdd {
			essential := ""
			// Find the essential value from the source template record.
			for _, re := range resolved {
				if recordKey(re.rec) == recordKey(toAdd[i]) {
					essential = re.tmplRec.Essential
					break
				}
			}
			if essential == "" {
				essential = "Always"
			}
			toAdd[i].DC = map[string]interface{}{
				"id":         uniqueID,
				"providerId": opts.ProviderID,
				"serviceId":  opts.ServiceID,
				"host":       host,
				"essential":  essential,
			}
		}
	}

	return &ProcessResult{
		ToAdd:    toAdd,
		ToDelete: toDelete,
	}, nil
}

// resolveHostName applies the DC host resolution rules:
// 1. "@" or empty -> if caller host is set, use host; otherwise "@" (apex)
// 2. FQDN detection: trailing dot -> strip domain suffix -> relative name
// 3. Non-@ non-FQDN -> append caller host if set
func resolveHostName(templateHost, callerHost, domain string) (string, error) {
	h := templateHost

	// Handle @ -> caller host or apex.
	if h == "@" || h == "" {
		if callerHost != "" && callerHost != "@" {
			return callerHost, nil
		}
		return "@", nil
	}

	// FQDN detection: trailing dot.
	if strings.HasSuffix(h, ".") {
		h = strings.TrimSuffix(h, ".")
		// Strip domain suffix if present.
		domainLower := strings.ToLower(domain)
		hLower := strings.ToLower(h)
		if hLower == domainLower {
			// FQDN equals domain -> always apex, regardless of caller host.
			return "@", nil
		}
		suffix := "." + domainLower
		if strings.HasSuffix(hLower, suffix) {
			relative := h[:len(h)-len(suffix)]
			return relative, nil
		}
		// FQDN that doesn't end in domain -> InvalidData.
		return "", fmt.Errorf("host %q is an FQDN outside zone %q", templateHost, domain)
	}

	// Non-FQDN, non-@: append caller host if present.
	if callerHost != "" && callerHost != "@" {
		return h + "." + callerHost, nil
	}
	return h, nil
}

// resolvePointsTo handles @ in pointsTo fields.
// For TXT, @ is literal. For others, @ -> fqdn.
func resolvePointsTo(val, callerHost, domain, typ string) string {
	if val != "@" {
		return val
	}
	// @ in pointsTo resolves to fqdn.
	if callerHost != "" && callerHost != "@" {
		return callerHost + "." + domain
	}
	return domain
}

// processSubstitute applies %var% replacement with DC error types.
func processSubstitute(in string, vars map[string]string, recIdx int, field string) (string, error) {
	if in == "" {
		return "", nil
	}
	var missErr error
	out := processVarRegex.ReplaceAllStringFunc(in, func(match string) string {
		name := match[1 : len(match)-1]
		v, ok := vars[name]
		if !ok {
			if missErr == nil {
				missErr = &MissingParameterError{Msg: fmt.Sprintf("record %d %s: missing variable %q", recIdx, field, name)}
			}
			return match
		}
		return v
	})
	if missErr != nil {
		return "", missErr
	}
	return out, nil
}

// checkUnclosedVar detects unclosed %variable tokens.
func checkUnclosedVar(s string) error {
	if s == "" {
		return nil
	}
	// Count % characters. If odd number outside of matched %var% pairs, it's unclosed.
	// Simple approach: remove all matched %var% pairs, then check for remaining lone %.
	cleaned := processVarRegex.ReplaceAllString(s, "")
	if strings.Contains(cleaned, "%") {
		// Check if there's an opening % without a closing %.
		idx := strings.Index(cleaned, "%")
		rest := cleaned[idx+1:]
		if !strings.Contains(rest, "%") {
			return fmt.Errorf("unclosed variable token")
		}
	}
	return nil
}

// processValidateHost validates a resolved host name per DC spec.
// Trailing dot is NOT allowed (DC spec: trailing dot in host = FQDN, which
// should have been resolved already; a trailing dot here means invalid).
func processValidateHost(host string) error {
	if host == "" || host == "@" {
		return nil
	}
	if strings.HasSuffix(host, ".") {
		return fmt.Errorf("trailing dot in host")
	}
	if strings.ContainsAny(host, " \t;") {
		return fmt.Errorf("host contains forbidden character")
	}
	if len(host) > 253 {
		return fmt.Errorf("host exceeds 253 characters")
	}

	// Validate wildcard position: * must be leftmost label only.
	labels := strings.Split(host, ".")
	for i, label := range labels {
		if strings.Contains(label, "*") {
			if i != 0 {
				return fmt.Errorf("wildcard not in leftmost position")
			}
			if label != "*" {
				return fmt.Errorf("wildcard must be a bare * label")
			}
			// Check that remaining labels are valid (no @ or * in non-first position).
			for j := 1; j < len(labels); j++ {
				if labels[j] == "*" || labels[j] == "@" {
					return fmt.Errorf("invalid label after wildcard: %q", labels[j])
				}
			}
		}
	}

	for _, label := range labels {
		if label == "*" || label == "@" {
			continue
		}
		if err := processValidateLabel(label); err != nil {
			return err
		}
	}
	return nil
}

// processValidateLabel validates a single DNS label.
func processValidateLabel(label string) error {
	if len(label) == 0 {
		return fmt.Errorf("empty DNS label")
	}
	if len(label) > 63 {
		return fmt.Errorf("DNS label exceeds 63 characters")
	}
	if label[0] == '-' || label[len(label)-1] == '-' {
		return fmt.Errorf("DNS label has leading or trailing hyphen")
	}
	for i := 0; i < len(label); i++ {
		c := label[i]
		ok := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_'
		if !ok {
			return fmt.Errorf("DNS label contains invalid character %q", c)
		}
	}
	return nil
}

// processValidateIP validates IP-like values for A/AAAA records.
// Lenient: accepts numeric-only dot-separated values for A (including 3-octet
// shorthand like 132.148.25), and hex-colon values for AAAA. Rejects hostnames,
// URLs, and whitespace.
func processValidateIP(typ, val string) error {
	if strings.ContainsAny(val, " \t") {
		return fmt.Errorf("contains whitespace")
	}
	switch typ {
	case "A":
		// Must contain only digits and dots, and at least one dot.
		if !strings.Contains(val, ".") {
			return fmt.Errorf("invalid IPv4 address")
		}
		for _, c := range val {
			if c != '.' && (c < '0' || c > '9') {
				return fmt.Errorf("invalid IPv4 address")
			}
		}
	case "AAAA":
		// Must contain a colon (IPv6), only hex digits, colons, and dots (mapped v4).
		if !strings.Contains(val, ":") {
			return fmt.Errorf("invalid IPv6 address")
		}
		for _, c := range val {
			ok := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') ||
				(c >= 'A' && c <= 'F') || c == ':' || c == '.'
			if !ok {
				return fmt.Errorf("invalid IPv6 address")
			}
		}
	}
	return nil
}

// validatePointsToLength checks FQDN length constraints on pointsTo.
func validatePointsToLength(val string) error {
	v := strings.TrimSuffix(val, ".")
	if len(v) > 253 {
		return fmt.Errorf("pointsTo exceeds 253 characters")
	}
	for _, label := range strings.Split(v, ".") {
		if len(label) > 63 {
			return fmt.Errorf("pointsTo label exceeds 63 characters")
		}
	}
	return nil
}

// processSPFM handles SPFM merge into existing zone SPF records.
func processSPFM(zone []entree.Record, spfm entree.Record) (add, del []entree.Record) {
	name := spfm.Name
	rules := spfm.Content

	// Find existing SPF TXT at the same name.
	var existingSPF *entree.Record
	for i := range zone {
		if nameEq(zone[i].Name, name) && zone[i].Type == "TXT" && strings.HasPrefix(zone[i].Content, "v=spf1 ") {
			existingSPF = &zone[i]
			break
		}
	}

	if existingSPF == nil {
		// Create new SPF record.
		ttl := spfm.TTL
		if ttl == 0 {
			ttl = 6000
		}
		add = append(add, entree.Record{
			Type:    "TXT",
			Name:    name,
			Content: "v=spf1 " + rules + " ~all",
			TTL:     ttl,
		})
		return
	}

	// Check if rule already present.
	if strings.Contains(existingSPF.Content, rules) {
		return nil, nil
	}

	// Merge: insert rules after "v=spf1 " and before the terminator.
	content := existingSPF.Content
	// Find the terminator (-all, ~all, ?all).
	terminator := ""
	for _, t := range []string{" -all", " ~all", " ?all"} {
		if strings.HasSuffix(content, t) {
			terminator = t
			content = strings.TrimSuffix(content, t)
			break
		}
	}
	if terminator == "" {
		terminator = " ~all"
	}

	// Insert after "v=spf1 ".
	prefix := "v=spf1 "
	rest := strings.TrimPrefix(content, prefix)
	newContent := prefix + rules + " " + rest + terminator

	del = append(del, *existingSPF)
	add = append(add, entree.Record{
		Type:    "TXT",
		Name:    name,
		Content: newContent,
		TTL:     existingSPF.TTL,
	})
	return
}

// checkInternalConflicts detects conflicts between records generated from the same template.
func checkInternalConflicts(resolved []resolvedEntry) error {
	// CNAME conflicts: two CNAMEs at the same name, or CNAME + non-CNAME at same name.
	// NS conflicts: NS + A/AAAA/CNAME/etc at same name (excluding other NS), or NS at name + record below it.
	type nameInfo struct {
		hasCNAME bool
		hasNS    bool
		hasOther bool // A, AAAA, MX, TXT, SRV, etc.
		cnameIdx int
	}
	names := make(map[string]*nameInfo)

	for i, re := range resolved {
		n := strings.ToLower(re.rec.Name)
		info, ok := names[n]
		if !ok {
			info = &nameInfo{cnameIdx: -1}
			names[n] = info
		}

		switch re.rec.Type {
		case "CNAME":
			if info.hasCNAME {
				return &InvalidDataError{Msg: fmt.Sprintf("record %d: duplicate CNAME at %q", i, re.rec.Name)}
			}
			if info.hasOther || info.hasNS {
				return &InvalidDataError{Msg: fmt.Sprintf("record %d: CNAME conflict at %q", i, re.rec.Name)}
			}
			info.hasCNAME = true
			info.cnameIdx = i
		case "NS":
			info.hasNS = true
			if info.hasCNAME {
				return &InvalidDataError{Msg: fmt.Sprintf("record %d: NS conflict with CNAME at %q", i, re.rec.Name)}
			}
			if info.hasOther {
				return &InvalidDataError{Msg: fmt.Sprintf("record %d: NS conflict with other record at %q", i, re.rec.Name)}
			}
		default:
			if re.rec.Type != "SPFM" && re.rec.Type != "TXT" && re.rec.Type != "REDIR301" && re.rec.Type != "REDIR302" {
				if info.hasCNAME {
					return &InvalidDataError{Msg: fmt.Sprintf("record %d: conflict with CNAME at %q", i, re.rec.Name)}
				}
				if info.hasNS {
					return &InvalidDataError{Msg: fmt.Sprintf("record %d: conflict with NS at %q", i, re.rec.Name)}
				}
				info.hasOther = true
			}
		}
	}

	// Check NS cascade: NS at name X conflicts with any non-NS record at name below X.
	for _, re := range resolved {
		if re.rec.Type != "NS" {
			continue
		}
		nsName := strings.ToLower(re.rec.Name)
		for _, other := range resolved {
			if other.rec.Type == "NS" {
				continue
			}
			otherName := strings.ToLower(other.rec.Name)
			if isChildOf(otherName, nsName) {
				return &InvalidDataError{Msg: fmt.Sprintf("NS at %q conflicts with %s at %q (below delegation)", re.rec.Name, other.rec.Type, other.rec.Name)}
			}
		}
	}

	return nil
}

// nameEq compares zone record names, treating "@" as apex.
func nameEq(a, b string) bool {
	normalize := func(s string) string {
		s = strings.ToLower(s)
		if s == "@" || s == "" {
			return "@"
		}
		return s
	}
	return normalize(a) == normalize(b)
}

// isChildOf returns true if child is a subdomain of parent.
// e.g., "www.bar" is a child of "bar", but "xbar" is not.
func isChildOf(child, parent string) bool {
	c := strings.ToLower(child)
	p := strings.ToLower(parent)
	if p == "@" || p == "" {
		// Everything is a child of the apex? No, only if child has labels.
		return c != "@" && c != "" && c != p
	}
	return strings.HasSuffix(c, "."+p)
}

// removeRecords removes all matching records from zone.
func removeRecords(zone []entree.Record, toRemove []entree.Record) []entree.Record {
	if len(toRemove) == 0 {
		return zone
	}
	removeSet := make(map[string]int) // key -> count to remove
	for _, r := range toRemove {
		k := recordKey(r)
		removeSet[k]++
	}
	var result []entree.Record
	for _, r := range zone {
		k := recordKey(r)
		if removeSet[k] > 0 {
			removeSet[k]--
			continue
		}
		result = append(result, r)
	}
	return result
}

func recordKey(r entree.Record) string {
	return fmt.Sprintf("%s|%s|%s|%d|%d|%d|%d|%s|%s",
		strings.ToUpper(r.Type),
		strings.ToLower(r.Name),
		r.Content,
		r.TTL,
		r.Priority,
		r.Weight,
		r.Port,
		r.Service,
		r.Protocol,
	)
}

// deduplicateRecords removes exact duplicates from a record slice.
func deduplicateRecords(recs []entree.Record) []entree.Record {
	if len(recs) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var result []entree.Record
	for _, r := range recs {
		k := recordKey(r)
		if seen[k] {
			continue
		}
		seen[k] = true
		result = append(result, r)
	}
	return result
}

// isValidRedirectURL does basic URL validation for REDIR targets.
func isValidRedirectURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	if u.Host == "" {
		return false
	}
	// Check for obviously malformed hosts.
	if strings.Count(u.Host, ":") > 1 {
		return false
	}
	return true
}

// normalizeRecord applies DC spec normalization:
// - Type is uppercased
// - Name is lowercased (except "@" preserved)
// - Content/data is lowercased for A, AAAA, CNAME, MX, NS, SRV (not TXT, custom)
func normalizeRecord(r entree.Record) entree.Record {
	r.Type = strings.ToUpper(r.Type)
	if r.Name != "@" {
		r.Name = strings.ToLower(r.Name)
	}
	switch r.Type {
	case "A", "AAAA", "CNAME", "MX", "NS", "SRV":
		r.Content = strings.ToLower(r.Content)
	}
	r.Protocol = strings.ToLower(r.Protocol)
	return r
}

type resolvedEntry struct {
	rec       entree.Record
	tmplRec   TemplateRecord
	origType  string
	isBacking bool // backing record for REDIR; skip conflict resolution
}
