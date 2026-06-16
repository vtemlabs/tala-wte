<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, paid training, paid CTF,
  or any for-profit use requires a license from VTEM Labs. See the LICENSE file.
-->
<script lang="ts">
  import GuidePage from '$lib/components/GuidePage.svelte';

  const DOC = `
## What this page is

The LDAP Directory is the embedded user store that backs enterprise wireless
authentication on this appliance. It runs a local OpenLDAP server (\`slapd\`)
that holds the accounts, groups, and credentials your simulated clients will
authenticate against.

![The LDAP Directory page](/guide/ldap.png)

In a WPA2-Enterprise or WPA3-Enterprise (802.1X) network, the access point does
not check the password itself. It hands the credentials to RADIUS, and RADIUS
validates them against a backend user store. On this appliance that backend is
this directory. The same directory also backs the optional credential-validation
mode of Open-network captive portals, where a harvested username and password
are checked for validity instead of being blindly accepted.

In short: edit a user here and you change who can join your enterprise networks
and whose harvested credentials will be accepted by a portal. See the
[RADIUS guide](/radius/guide) for how the authentication path is wired together,
and the [Networks guide](/networks/guide) for where enterprise networks are
configured.

## Directory status

The header shows a \`slapd running\` or \`slapd stopped\` badge. Authentication
only works when the badge reads **running**. If it reads stopped, no enterprise
client and no portal credential check will succeed.

Below the header a status strip reports the directory's coordinates:

- **Base DN** - the root of the directory tree, for example
  \`dc=acmecorp,dc=local\`. Every user and group lives under this DN.
- **Bind DN** - the administrative account RADIUS and the portal use to read the
  directory, shown as \`cn=admin,<base DN>\`.
- **Port** - the LDAP service port, \`3389\`. This is a local, appliance-internal
  service.
- **Users** - the current count of accounts in the directory.

## Directory Provisioning

Provisioning **wipes and rebuilds the entire directory** with a fresh set of
generated users, groups, and credentials. Use it to stand up a believable
corporate directory in one click instead of adding accounts by hand. Every
provisioning action asks for confirmation first, because it destroys the
existing contents.

There are three ways to provision:

### Reset to Default (ACME Corp)

Rebuilds the directory as the standard demo company: **ACME Corp**, domain
\`acmecorp.local\`, **15 users**, using the realistic password mix described
below. This is the known-good baseline to return to when a directory has been
edited beyond recognition.

### Generate Random Company

Builds a fresh directory for a randomly generated company name and domain, with
a generated set of users and credentials. Use it when you want a different,
unfamiliar directory for an exercise rather than the always-the-same ACME Corp.

### Custom

Opens an inline form so you can specify the directory yourself:

- **Company Name** - required. Used for the organization and in generated names.
- **Email Domain** - required, for example \`contoso.local\`. Generated user
  email addresses use this domain.
- **Users** - how many accounts to generate, from 1 to 50.

A toggle, **All Strong Random Passwords**, controls how credentials are
generated:

- **Off (recommended)** - a realistic corporate password mix: roughly 40 percent
  weak (think \`Password1!\`, \`Welcome123\`), roughly 30 percent semi-personal
  (a first name plus a year), and roughly 30 percent strong random. This is what
  you want for a credible cracking or harvesting exercise, because real
  directories contain weak passwords.
- **On** - every user gets a unique 12-character random password. Choose this
  only when you specifically want a directory full of password-manager-grade
  accounts.

Press **Provision** to build the directory.

### Provisioning results

After any provisioning run, a result table lists the generated accounts with
their **UID**, **name**, **email**, and **password** in plaintext. This is your
record of the credentials that now exist, so copy down anything you need for the
exercise before navigating away.

## Managing users

Below the provisioning panel, the **Users** tab lists every account and lets you
add or remove them individually.

### Adding a user

The add-user row has five fields:

- **UID** (required) - the login name, for example \`jdoe\`. This is what a client
  or portal submits as the username.
- **CN (Full Name)** (required) - the display name, for example \`John Doe\`.
- **SN (Last Name)** - the surname, for example \`Doe\`.
- **Email** - the user's email address, for example \`jdoe@tala.wte\`.
- **Password** (required) - the credential the user authenticates with.

UID, CN, and Password are mandatory; the **Add User** button stays disabled until
all three are filled. Submitting adds the account to the live directory
immediately, so a client can authenticate with it right away.

### Viewing and copying passwords

Each row shows the user's password masked behind dots. For accounts stored as
plaintext you can:

- **Show / Hide** - reveal or re-mask the password in place.
- **Copy** - copy the password to your clipboard so you can paste it into a
  client supplicant or portal form.

Passwords that \`slapd\` stored as a one-way hash (rendered as \`(hashed)\`)
cannot be revealed or copied, because the original value is not recoverable.
Provisioned and manually added accounts are stored as plaintext so they remain
usable for exercises.

> Clipboard copy can fail in a non-secure browser context. If the copy button
> reports an error, reveal the password with Show and copy it manually.

### Deleting a user

The **Del** action on a row removes that account from the directory after a
confirmation prompt. A deleted user can no longer authenticate to enterprise
networks or pass a portal credential check.

## Groups

The **Groups** tab lists the directory's groups and lets you create new ones.
Enter a **Group CN** (for example \`wifi-users\`) and press **Create Group**.
Each group card shows the group's common name, its full DN, and any members it
contains. Groups are useful for organizing accounts and for any policy that
keys off group membership.

## Test Auth

The **Test Auth** tab is a quick credential check that runs a real bind against
the directory, exactly the way RADIUS does during 802.1X. Use it to confirm a
username and password actually work **before** a client tries them, so you can
tell a typo apart from a genuine authentication failure.

1. Enter a **Username (UID)** and **Password**.
2. Press **Test Authentication**.

A green **Authentication Successful** result confirms the credentials bind and
shows the matched user DN. A red **Authentication Failed** result means the bind
was rejected, and the message explains why (wrong credentials, no such user, or
the directory being unreachable because \`slapd\` is stopped).

## How it ties together

This directory is the single source of truth for "who is allowed in" across the
appliance:

- **Enterprise wireless (802.1X)** - WPA2-Enterprise and WPA3-Enterprise
  networks authenticate clients through RADIUS, and RADIUS validates the
  submitted credentials against this directory. A user that exists and binds
  here is a user that can join the network. See the [RADIUS guide](/radius/guide).
- **Open-network captive portals** - a portal configured to validate harvested
  credentials checks the captured username and password against this directory,
  so only credentials that are real (present and correct in the directory) are
  accepted. See the [Networks guide](/networks/guide).

Provision a believable directory, verify a couple of accounts on the Test Auth
tab, and your enterprise networks and credential-checking portals are ready to
exercise against real, known credentials.
`;
</script>

<GuidePage
  title="LDAP Directory Guide"
  subtitle="The embedded directory that backs enterprise authentication"
  backHref="/ldap"
  backLabel="Back to LDAP"
  crumb="LDAP Directory"
  doc={DOC}
/>
