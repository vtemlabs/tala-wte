A credential set is a list of valid logins for one captive-portal auth type. Assign a set to a network's portal and submissions are checked against it: a match grants access, a wrong entry is rejected and re-prompted, just like a real hotel front desk or corporate login. Sets exist so that the validating portal types are actually validatable, not just data collectors.

Credential sets are managed in the Credential Sets panel on the Captive Portals page. See [[Captive-Portals]] for the portal side and [[Networks]] for the network side.

## When a credential set applies

Only the five validating auth types use a credential set:

- Username & password
- Email & password
- Hotel (room + last name)
- Voucher / access code
- Membership (ID + PIN)

The three collect-only types (Click-through, Email capture, Information form) do not validate and need no set; they grant access on submit and simply record what was entered.

## Generating a set

![The Credential Sets panel](images/portals-credentials.png)

In the Credential Sets panel on the Captive Portals page:

1. Pick the auth type (only the validating types appear in the dropdown).
2. Set a count (prefilled at 25; 1 to 1000 are accepted).
3. Optionally give it a name (for example "Harbor Hotel Guests"); blank defaults to the auth type's label plus " set".
4. Click "Generate set". Tala WTE creates that many believable, validatable, de-duplicated entries.

The panel lists every set with its name, auth type badge, and entry count. View shows every entry in a table; Del removes a set.

What gets generated per type:

- Hotel - a last name plus a room number.
- Voucher - an 8-character access code (with a hyphen).
- Membership - a member ID like `M1234567` plus a 4-digit PIN.
- Username & password - a username (first initial + last name) plus a generated password.
- Email & password - an email plus a generated password.

## Auto-generation on start

You usually do not have to generate anything by hand. When a validating portal network starts and no credential set is assigned, the leader auto-generates a 25-entry set, names it "<SSID> - <auth type label> (auto)", and assigns it to that network. A deployed pack member then receives a working credential from that set and passes the portal automatically, so a hotel SSID "just works" out of the box and Captured Data fills with believable logins on its own.

A network that already has a set assigned, and any non-validating portal, are left untouched.

## Assigning a set on the Networks form

On the new-network or edit form, with Security Protocol set to Open and the Captive Portal Sandbox enabled:

- Choose the portal under Portal Module.
- If the portal validates, a "Credential set" selector appears, labeled with the auth type (for example "Credential set (membership validation)").
  - The first option is "No set - capture only": record submissions without checking them (still grants on submit).
  - Otherwise pick one of the sets that match the portal's auth type (only matching-type sets are listed).
  - If no matching set exists yet, the form shows a "Generate a ... credential set" link to the Captive Portals page instead of a dropdown; until you make one, the portal captures and grants on submit.

Only sets whose auth type matches the selected portal's auth type are offered, so you cannot assign a hotel set to a voucher portal.

## Field-alias flexibility

A credential set is stored against canonical field names (for example `room_number`, `last_name`, `code`, `username`, `password`, `member_id`, `pin`, `email`), but real portal templates name their form fields differently. Tala WTE matches submitted field names through a set of aliases, so one set validates across template variants. For example:

- `room_number` also matches `room`, `stateroom`, `unit`.
- `last_name` also matches `lastname`, `surname`.
- `code` also matches `voucher`, `access_code`, `survey_code`, `ticket_number`, `bed_code`, `promo_code`.
- `username` also matches `account`, `login`, `user`, `userid`.
- `member_id` also matches `rewards_number`, `loyalty_id`, `card_number`, `membership_id`, `account_number`.
- `password` also matches `passcode`, `passphrase`.

So a single Hotel set works whether the form field is named `room_number` or `stateroom`. When a submission is checked, non-secret fields match case-insensitively and `password`/`pin` must match exactly; an empty submitted key field never matches.

## Tip

Use View on a set to read out a valid login if you want to test a portal by hand. For a hands-off credentialed lab, just start the network and deploy a pack member: the auto-generated set plus the member populate Captured Data for you.
