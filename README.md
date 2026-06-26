# pp-zapvend-cli

ZapVend electricity vending CLI.

## VPS wrapper

The production VPS installs `scripts/pp-zapvend-cli` to `/usr/local/bin/pp-zapvend-cli`.
The wrapper sources `/root/.hermes/.env`, supports global flags before the
subcommand, and uses the public ZapVend API when `ZAPVEND_CLI_SECRET` and
`ZAPVEND_CONFIG` are intentionally absent.

Install/update:

```bash
install -m 0755 scripts/pp-zapvend-cli /usr/local/bin/pp-zapvend-cli
go build -o /root/go/bin/pp-zapvend-cli .
```

Safe demo checks:

```bash
pp-zapvend-cli vend anita 50 --demo --json
pp-zapvend-cli --json vend anita 50 --demo
```
