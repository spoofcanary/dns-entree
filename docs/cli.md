# entree CLI Reference

Auto-generated from cobra command definitions. Do not edit by hand.
Regenerate with `make docs`.

## entree

DNS provider CLI (agent-friendly)

### Synopsis

entree is a DNS provider CLI for detection, record push, verification, SPF merge, and Domain Connect templates.

Exit codes: 0 success, 1 runtime error, 2 user error.

### Options

```
  -h, --help   help for entree
```


---

## entree apply

Apply records or a Domain Connect template to a domain

```
entree apply <domain> [flags]
```

### Options

```
      --cache-dir string     template cache directory (apply --template)
      --dry-run              compute diff without writing
  -h, --help                 help for apply
      --record stringArray   record spec TYPE:NAME:VALUE (repeatable)
      --template string      Domain Connect template providerId/serviceId (05-03)
      --var stringArray      template variable key=value (repeatable)
      --vars-file string     template variables JSON file
```


---

## entree dc-discover

Probe a domain for Domain Connect v2 support

```
entree dc-discover <domain> [flags]
```

### Options

```
  -h, --help   help for dc-discover
```


---

## entree detect

Detect the DNS hosting provider for a domain

```
entree detect <domain> [flags]
```

### Options

```
  -h, --help   help for detect
```


---

## entree spf-merge

Merge SPF includes into an existing record (pure, no I/O)

```
entree spf-merge <current> <include> [<include>...] [flags]
```

### Options

```
  -h, --help   help for spf-merge
```


---

## entree templates

Manage Domain Connect templates (sync, list, show, resolve)

### Options

```
      --cache-dir string   template cache directory (default XDG cache)
  -h, --help               help for templates
```


---

## entree templates list

List cached Domain Connect templates

```
entree templates list [flags]
```

### Options

```
  -h, --help   help for list
```

### Options inherited from parent commands

```
      --cache-dir string   template cache directory (default XDG cache)
```


---

## entree templates resolve

Resolve a template with --var/--vars-file and print the records

```
entree templates resolve <providerID>/<serviceID> [flags]
```

### Options

```
  -h, --help               help for resolve
      --var stringArray    template variable key=value (repeatable)
      --vars-file string   JSON file of template variables
```

### Options inherited from parent commands

```
      --cache-dir string   template cache directory (default XDG cache)
```


---

## entree templates show

Print a template as JSON

```
entree templates show <providerID>/<serviceID> [flags]
```

### Options

```
  -h, --help   help for show
```

### Options inherited from parent commands

```
      --cache-dir string   template cache directory (default XDG cache)
```


---

## entree templates sync

Clone or fast-forward the Domain Connect templates repo

```
entree templates sync [flags]
```

### Options

```
  -h, --help   help for sync
```

### Options inherited from parent commands

```
      --cache-dir string   template cache directory (default XDG cache)
```


---

## entree verify

Query authoritative NS for a record and report the value

```
entree verify <domain> <type> <name> [flags]
```

### Options

```
      --contains string   case-insensitive substring match on record value
  -h, --help              help for verify
```


---

