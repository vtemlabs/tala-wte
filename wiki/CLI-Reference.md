Tala WTE is a single binary. Day to day you only ever run one of its lifecycle subcommands (`install`, `install-client`, `uninstall`); `serve` is what the systemd unit runs, and you rarely invoke it by hand. Everything else (creating networks, portals, captures, updates) happens in the web console.

For the install walkthrough see [[Installation]]; for routine updates see [[Updating]].

## Important: do not run subcommands against a live service

Running any subcommand against the same data dir while the `tala-wte.service` systemd service is up boots the full app lifecycle and, on exit, fires the shutdown teardown, which deletes every `wte-*` network namespace and kills all running networks and access points (including the live service's). The CLI process and the running service share the same lifecycle hooks, and the exit teardown is global, not scoped to the CLI's own process.

So: do not stand up superusers or run other verbs on a running box from the CLI. Use the web UI / API instead. If you must run a subcommand, expect to restart the affected networks afterward, or stop the service first.

## tala-wte install

```
sudo ./tala-wte-linux-arm64 install
```

Installs Tala WTE in the access-point (server) role as a systemd service. Verifies and installs dependencies, installs USB wireless recovery, copies the binary into `/var/lib/tala-wte`, writes and starts `tala-wte.service`, waits for `:8443`, and prints the console URL. Takes no flags and is idempotent; re-run any time to upgrade the binary or repair the unit. The database under `/var/lib/tala-wte` is preserved across reinstalls. Must run as root. It never creates an account; admin setup is done in the browser (see [[Installation]]).

`-h` / `--help` prints usage.

## tala-wte install-client

```
sudo ./tala-wte-linux-arm64 install-client
```

Installs Tala WTE in the client role: the box joins another Tala WTE access point from an imported config and generates traffic. Same binary, data dir, and `tala-wte.service` unit as the server role, but the unit sets `TALA_MODE=client` so the console shows the client view. Installs the lighter client dependency set and heals wedged adapters. Idempotent; must run as root. You can also flip roles later from [[Settings]] without reinstalling. See [[The-Pack]].

`-h` / `--help` prints usage.

## tala-wte uninstall

```
sudo tala-wte uninstall
sudo tala-wte uninstall --purge
```

Stops and disables the service and removes the unit file. Without `--purge` it preserves `/var/lib/tala-wte` (the database and all captures). With `--purge` it also deletes `/var/lib/tala-wte` and wipes the terminal session logs and recorder binaries; there is no undo. Must run as root.

`-h` / `--help` prints usage.

## tala-wte serve

```
tala-wte serve
```

Runs the service in the foreground: starts the embedded OpenLDAP, binds PocketBase to loopback (`127.0.0.1:8090`), fronts it with the TLS reverse proxy on `:8443` (and the `:80`-to-`:8443` redirect), and verifies dependencies on the way up. This is the `ExecStart` line in the systemd unit; you normally let systemd run it (`systemctl status tala-wte`, `journalctl -u tala-wte`) rather than invoking it yourself. Running a second `serve` against the same data dir while the service is up will collide and tear down running networks on exit (see the warning above).

## Other verbs (inherited from PocketBase)

Tala WTE intercepts `install`, `install-client`, and `uninstall` itself; any other argument falls through to the embedded PocketBase root command (for example `--version`, `migrate`, and `superuser`). These are PocketBase built-ins, not Tala WTE features, and most are unnecessary in normal use:

- Do not use `superuser` to create or reset admin accounts on a live box. First-boot admin creation is the browser setup wizard (see [[Installation]]); running `superuser` against a running service triggers the teardown described above and kills running networks. Manage operators through the console.
- Migrations run automatically at startup (auto-migrate is enabled), so you do not run `migrate` by hand.

## Releasing (maintainers)

Releases are not a runtime CLI verb. The version is stamped from the git tag at build time: `scripts/bump-version.sh [patch|minor|major|X.Y.Z] [beta]` validates the tree, computes and writes an annotated tag, and (by default) pushes it, which triggers the GitHub Actions release workflow to build and publish the `linux/amd64` and `linux/arm64` binaries. Use `--no-push` / `--local` to tag locally only and `--dry-run` to preview. End users do not need this; they update from the console (see [[Updating]]).
