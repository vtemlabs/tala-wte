Tala WTE runs a real OpenLDAP (slapd) directory that acts as your fake company. It is the user store RADIUS checks for WPA-Enterprise logins, and it can also back captive-portal "Require Login" validation. Populate it once and your enterprise and portal labs have believable people to authenticate as.

The page is reached from the sidebar (LDAP Directory). A status badge in the header reads "slapd running" or "slapd stopped". A stat strip across the top shows the directory's fixed coordinates and current user count.

![The LDAP Directory page](images/ldap.png)

## Directory coordinates

These values are baked in and shown in the stat strip:

- Base DN: `dc=tala,dc=wte`
- Bind DN: `cn=admin,dc=tala,dc=wte` (the admin account the app binds as for all writes)
- Port: `3389` (slapd listens on `ldap://127.0.0.1:3389`)
- Users: a live count of accounts under `ou=Users`

The admin bind password is generated and persisted on disk the first time it is needed (or taken from the `TALA_LDAP_ADMIN_PASSWORD` environment variable); you never set it by hand. Users live under `ou=Users,dc=tala,dc=wte` as `inetOrgPerson` entries, groups under `ou=Groups,dc=tala,dc=wte` as `groupOfNames` entries.

## Provisioning a company

![Directory provisioning](images/ldap-provision.png)

The provisioning panel sits above the tabs. Every provision wipes the directory and rebuilds it from scratch, then shows you the generated users and their passwords in a table (UID, Name, Email, Password). There are three ways to fill the directory.

### Generate Random Company

The fastest realistic start. It picks one of a set of believable company names (for example "Vanguard Industries", "Meridian Systems", "Citadel Infosec") with a matching `.local` domain, creates 10 to 20 users spread across real departments (Engineering, Sales, Information Technology, Finance, Marketing, HR, and so on) with real job titles, and gives each user a realistic password mix: roughly 40 percent weak (the `Password1!`, `Welcome123` kind), 30 percent semi-personal (a first name plus a year), and 30 percent strong random. Use this when you want a directory that feels like a real domain and gives cracking exercises something to chew on. This is the default mode, so the generated passwords are deliberately not all strong.

### Reset to Default (ACME Corp)

Rebuilds a fixed baseline: company "ACME Corp", domain `acmecorp.local`, 15 users, using the same realistic (mixed, not all-strong) password distribution. Use this when you want a known, repeatable directory across runs, or to get back to a clean baseline after experimenting. This is the same baseline the enterprise preflight uses when it auto-provisions LDAP for you (see [[RADIUS-802.1X]]).

### Custom

Click Custom to open the inline form, then set:

- Company Name (required) and Email Domain (required)
- Users: a count from 1 to 50. Values above 50 are capped at 50.
- All Strong Random Passwords toggle:
  - Off (recommended, the default): every user gets the realistic corporate mix described above. Pick this when the lesson involves cracking or guessing passwords, because you want some weak accounts in the set.
  - On: every user gets a unique 12-character random password. Pick this when you want authentication to always succeed and do not care about cracking, for example when you only need the directory to back a working enterprise login.

Click Provision to wipe and rebuild.

## Users

![The Users tab](images/ldap-users.png)

The Users tab lists every account. Columns are UID (`uid`), Name (`cn`), Title, Department, Email (`mail`), Password, and a row actions column. Filter the list with the field above the table (it matches across uid, name, email, title, and department) and sort by clicking the UID, Name, Title, or Department headers (click again to flip the direction).

The Password column handles two cases. A plaintext password (the generated kind) shows as dots with Show and Copy buttons next to it: Show reveals it inline, Copy puts it on the clipboard. This is how you grab a working credential to set up a test client. A hashed value (one stored with a `{SSHA}`-style prefix) cannot be recovered, so it reads "(hashed)" with no reveal or copy.

Add a user with the inline row at the top of the tab: UID, Full name, Last name, Email, Password, then Add User. UID, Full name, and Password are required; Last name and Email are optional.

Per row:

- Set pw prompts for a new password and applies it. Use it to make a weak account strong, or to set a known credential on an account so you can log in from a test client.
- Del removes the user (with a confirm prompt).

## Groups

![The Groups tab](images/ldap-groups.png)

The Groups tab shows the directory's groups, which a generated company populates realistically so a trainee enumerating the domain finds what they expect:

- Domain Users (everyone) and Domain Admins (IT staff plus executives, capped)
- One group per populated department (Engineering, Sales, Information Technology, Finance, and so on)
- Operator groups derived from IT (Help Desk, Backup Operators, File Server Admins) and an Executives group
- Access groups (VPN Users, Remote Desktop Users) as realistic subsets of the company
- wifi-users (everyone) and wifi-admins (admins), the Wi-Fi groups the RADIUS path expects

Columns are Group, Members (a count), Membership, and actions. Filter by group name or by member uid (so you can find which groups a given user is in), and sort by Group name or Members count.

Membership is managed inline. Each current member shows as a clickable chip; clicking the chip removes that user from the group. To add a member, type a uid into the "add uid" field in the row and click Add (or press Enter). Create a new group with the inline field at the top of the tab (the new group CN, then Create Group); Del removes a group. Group membership is what you reference when you scope access in a lesson.

## Test Auth

![Test Auth](images/ldap-testauth.png)

Before you stand up an enterprise network, use the Test Auth tab to confirm a credential works end to end. Enter a Username (UID) and Password and click Test Authentication. Tala WTE performs a real LDAP bind as `uid=<user>,ou=Users,dc=tala,dc=wte` and reports success (with the bound DN) or failure (with the reason). An email-style entry is reduced to its local part before binding, so `jsmith@acmecorp.local` is tried as `jsmith`. This is the quickest way to verify the EAP Identity and Password you plan to put on an enterprise network are real.

## What it backs

- WPA2/WPA3-Enterprise (802.1X) networks: RADIUS validates each client's directory login against this directory. See [[RADIUS-802.1X]] for the auth chain and [[Networks]] for creating enterprise networks. The EAP Identity and Password you set on an enterprise network must be a real user here, so verify it on Test Auth first.
- Captive-portal "Require Login (Directory / LDAP)": when enabled on an Open network's captive portal, portal logins are validated against this same directory like a corporate hotspot.

## Tips

- Provision the directory before starting a WPA-Enterprise network or a "Require Login" portal; both authenticate against it.
- Leave the mixed-password setting on (All Strong Random Passwords off) for realistic cracking labs; flip it on when you want auth to always succeed.
- An empty database is bootstrapped automatically with the ACME Corp baseline (15 users, 2 groups) the first time slapd starts, so the directory is never empty when you arrive.
