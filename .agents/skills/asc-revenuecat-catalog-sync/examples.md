# Examples: asc RevenueCat catalog sync

Use these examples as execution templates for realistic catalog synchronization workflows.

## Example 1: Drift audit only (read-only)

Goal: compare ASC and RevenueCat, produce a no-write reconciliation report.

### User request
`Audit my ASC subscriptions and IAP catalog against RevenueCat and show what is missing on either side.`

### Expected behavior
1. Read ASC:
   - `asc subscriptions groups list --app "APP_ID" --paginate --output json`
   - `asc iap list --app "APP_ID" --paginate --output json`
   - `asc subscriptions list --group-id "GROUP_ID" --paginate --output json` (for each group)
2. Read RevenueCat via MCP:
   - list apps/products/entitlements/offerings/packages for `project_id`
3. Build and present a diff:
   - missing in ASC
   - missing in RevenueCat
   - type mismatch
   - app/platform mismatch
4. Stop for confirmation (no writes).

## Example 2: Create missing ASC subscriptions, then map to RevenueCat

Goal: bootstrap both systems when store products are partially missing.

### User request
`Ensure monthly and annual subscriptions exist in ASC for app APP_ID, then sync them to RevenueCat project PROJECT_ID under my iOS app.`

### Expected behavior
1. Audit both catalogs, present the no-write plan, and request confirmation.
2. Resolve the ASC parents by exact identifiers with fully paginated reads:
   - `asc subscriptions groups list --app "APP_ID" --paginate --output json`
   - `asc subscriptions list --group-id "GROUP_ID" --paginate --output json`
   - zero exact matches means create, one means reuse its ID, and more than one
     means stop for an explicit ID. Follow [Step D](SKILL.md#step-d---ensure-missing-asc-items-if-requested)
     for every parent and mutable version; never create another resource to
     resolve ambiguity.
3. For each missing subscription, use its own setup inputs to reconcile the
   review screenshot, complete price matrix, and requested sale availability
   before version metadata:

   ```bash
   # CONDITIONAL PSEUDOCODE: audit each exact productId, then run only its branch.
   # Monthly: zero matches -> create with setup; one -> reuse its SUB_ID or run
   # these same inputs only for explicitly approved reconciliation; many -> stop.
   asc subscriptions setup \
     --app "APP_ID" \
     --group-id "GROUP_ID" \
     --reference-name "Monthly" \
     --product-id "com.example.premium.monthly" \
     --subscription-period ONE_MONTH \
     --review-screenshot "./monthly-review.png" \
     --price "3.99" \
     --price-territory "USA" \
     --territories "USA" \
     --no-verify \
     --output json

   # Annual: zero matches -> create with setup; one -> reuse its SUB_ID or run
   # these same inputs only for explicitly approved reconciliation; many -> stop.
   asc subscriptions setup \
     --app "APP_ID" \
     --group-id "GROUP_ID" \
     --reference-name "Annual" \
     --product-id "com.example.premium.annual" \
     --subscription-period ONE_YEAR \
     --review-screenshot "./annual-review.png" \
     --price "39.99" \
     --price-territory "USA" \
     --territories "USA" \
     --no-verify \
     --output json
   ```

   Reuse an existing subscription unless the user explicitly approves its
   reconciliation. Use `--repair` only when the same setup inputs must rebuild
   a stale complete price matrix.
4. Resolve zero/one/many `PREPARE_FOR_SUBMISSION` group and subscription
   versions, then create or update their version-scoped localizations as shown
   in Step D. Use `Premium Monthly` with a monthly-specific description for the
   monthly version and `Premium Annual` with an annual-specific description for
   the annual version. Read back both selected versions and gate all
   subscriptions before any RevenueCat write:

   ```bash
   mkdir -p "./audit"
   asc validate subscriptions --app "APP_ID" --strict --output json --pretty \
     > "./audit/subscriptions-validation.json"
   ```

5. Only after the validator exits zero, apply the confirmed RevenueCat plan:
   - create app if missing (`type: app_store`, same bundle identifier)
   - create products with matching `store_identifier`
   - create entitlement (for example `premium`) and attach products
   - optionally create `default` offering with `$rc_monthly` and `$rc_annual`
6. Re-read both catalogs and return the final summary and failures list.

## Example 3: Sync one-time IAP and keep consumables entitlement-free

Goal: model recommended entitlement behavior by product type.

### User request
`Sync my non-consumable lifetime IAP and consumable coin pack to RevenueCat.`

### Expected behavior
1. Audit both catalogs, confirm product type mapping, present the plan, and
   request confirmation:
   - `NON_CONSUMABLE` -> `non_consumable`
   - `CONSUMABLE` -> `consumable`
2. Resolve each IAP by exact `productId` with
   `asc iap list --app "APP_ID" --paginate --output json`. Zero exact matches
   means create, one means reuse its ID, and more than one means stop for an
   explicit `IAP_ID`. For each missing IAP, create only its own parent:

   ```bash
   asc iap create \
     --app "APP_ID" \
     --type NON_CONSUMABLE \
     --ref-name "Lifetime" \
     --product-id "com.example.lifetime" \
     --output json

   asc iap create \
     --app "APP_ID" \
     --type CONSUMABLE \
     --ref-name "Coins 100" \
     --product-id "com.example.coins.100" \
     --output json
   ```

3. Audit pricing and availability separately for each resolved IAP before
   reconciling either one:

   ```bash
   asc iap pricing summary --iap-id "LIFETIME_IAP_ID" --output json
   asc iap pricing availability view --iap-id "LIFETIME_IAP_ID" --output json
   # On a successful parent view, capture .data.id, then:
   asc iap pricing availabilities available-territories --id "LIFETIME_AVAILABILITY_ID" --paginate --output json
   asc iap pricing summary --iap-id "COINS_IAP_ID" --output json
   asc iap pricing availability view --iap-id "COINS_IAP_ID" --output json
   # On a successful parent view, capture .data.id, then:
   asc iap pricing availabilities available-territories --id "COINS_AVAILABILITY_ID" --paginate --output json
   ```

   Only an App Store Connect `404` from the pricing summary proves that a price
   schedule is absent. Stop and report authentication, transport, decoding, and
   every other summary error. For availability, capture `.data.id` from each
   successful parent view and use it for the fully paginated territory read. A
   parent-view `404` means availability is missing; every other parent or
   territory-read error stops that item. Do not infer the territory set from
   parent-view relationship data because that relationship is optional and may
   be paged. Compare the complete returned territory-ID set with that product's
   complete approved set, ignoring order.
   After confirmation, create a missing schedule or set availability only when
   it is missing or that complete set differs:

   ```bash
   # CONDITIONAL PSEUDOCODE: do not run this block as a sequence.
   # If the lifetime pricing summary returned the confirmed missing-schedule 404:
   asc iap pricing schedules create --iap-id "LIFETIME_IAP_ID" --base-territory "USA" --prices "LIFETIME_PRICE_POINT_ID" --output json
   # If lifetime availability is missing or its complete territory set differs:
   asc iap pricing availability set --iap-id "LIFETIME_IAP_ID" --territories "USA" --output json
   # Otherwise reuse the matching lifetime pricing and availability.

   # If the coin pricing summary returned the confirmed missing-schedule 404:
   asc iap pricing schedules create --iap-id "COINS_IAP_ID" --base-territory "USA" --prices "COINS_PRICE_POINT_ID" --output json
   # If coin availability is missing or its complete territory set differs:
   asc iap pricing availability set --iap-id "COINS_IAP_ID" --territories "USA" --output json
   # Otherwise reuse the matching coin pricing and availability.
   ```

4. Resolve zero/one/many mutable versions and finish each product's own
   version-scoped localization and review image. Every create below is
   conditional on the preceding fully paginated read returning zero matches:

   ```bash
   # CONDITIONAL PSEUDOCODE: audit each list, then run only its matching branch.
   asc iap versions list --iap-id "LIFETIME_IAP_ID" --state PREPARE_FOR_SUBMISSION --paginate --output json
   # If zero versions, create; if one, reuse; if more than one, stop:
   asc iap versions create --iap-id "LIFETIME_IAP_ID" --output json
   asc iap versions localizations list --version-id "LIFETIME_VERSION_ID" --paginate --output json
   # If zero localizations, create; if one differs, update it by its resolved ID;
   # if one matches, reuse; if more than one, stop:
   asc iap versions localizations create --version-id "LIFETIME_VERSION_ID" --locale "en-US" --name "Lifetime" --description "Unlock lifetime access." --output json
   asc iap versions images list --version-id "LIFETIME_VERSION_ID" --paginate --output json
   # If zero images, upload; if one proven match, reuse; otherwise stop:
   asc iap versions images create --version-id "LIFETIME_VERSION_ID" --file "./lifetime-review.png" --output json

   asc iap versions list --iap-id "COINS_IAP_ID" --state PREPARE_FOR_SUBMISSION --paginate --output json
   # If zero versions, create; if one, reuse; if more than one, stop:
   asc iap versions create --iap-id "COINS_IAP_ID" --output json
   asc iap versions localizations list --version-id "COINS_VERSION_ID" --paginate --output json
   # If zero localizations, create; if one differs, update it by its resolved ID;
   # if one matches, reuse; if more than one, stop:
   asc iap versions localizations create --version-id "COINS_VERSION_ID" --locale "en-US" --name "Coins 100" --description "Add 100 coins." --output json
   asc iap versions images list --version-id "COINS_VERSION_ID" --paginate --output json
   # If zero images, upload; if one proven match, reuse; otherwise stop:
   asc iap versions images create --version-id "COINS_VERSION_ID" --file "./coins-100-review.png" --output json
   ```

   Update an existing localization only when its resolved values differ. For an
   image, zero matches means upload and one proven matching image means reuse.
   More than one match, a differing image, or an image that cannot be proven to
   match must stop and be reported: `images update` changes upload state only,
   it cannot replace the file, and this workflow never deletes resources.

5. Gate all resolved IAPs before any RevenueCat creation or attachment:

   ```bash
   mkdir -p "./audit"
   asc validate iap --app "APP_ID" --strict --output json --pretty \
     > "./audit/iap-validation.json"
   ```

6. Only after the validator exits zero, apply the confirmed RevenueCat plan:
   - create both products
   - attach **non-consumable** product to an entitlement (for example `lifetime_access`)
   - skip entitlement attachment for consumable by default (unless user explicitly asks)
7. Re-read both catalogs and return created/skipped/failed counts.

## Example 4: Controlled apply in CI-style automation

Goal: make apply mode safe and repeatable in team workflows.

### User request
`Run a full sync and apply missing resources.`

### Expected behavior
1. Run the fully paginated ASC and RevenueCat audit and print a no-write plan.
2. Request explicit approval before any ASC or RevenueCat create/update.
3. Apply each approved item in deterministic order:
   - resolve ASC parents and mutable versions with the zero/one/many rules in
     Step D
   - preserve each product's own identifiers, names, periods, prices, locales,
     and image paths; never reuse one product's template values for another
   - reconcile version-scoped metadata and review screenshots/images; upload a
     missing image, reuse a proven match, and stop on a differing or ambiguous
     image because this no-delete workflow cannot replace it
   - read IAP pricing summary and availability separately; only a summary `404`
     means no schedule, and only an availability-parent `404` means missing
     availability; every other error stops that item
   - capture the availability ID, fully paginate
     `asc iap pricing availabilities available-territories --id "AVAILABILITY_ID" --paginate --output json`,
     and set availability only when it is missing or the complete approved and
     current territory-ID sets differ
   - when subscriptions exist in the plan, save and run
     `asc validate subscriptions --app "APP_ID" --strict --output json --pretty`;
     map no subscriptions unless this subscription-class gate exits zero
   - when IAPs exist in the plan, save and run
     `asc validate iap --app "APP_ID" --strict --output json --pretty`; map no
     IAPs unless this IAP-class gate exits zero
   - when both classes exist, run both validators; a failed class is skipped,
     the other class may continue only if its own gate exits zero, and all gate
     and per-item failures are aggregated
   - RC app/product for items in each successfully gated class
   - RC entitlement + attachments
   - RC offering/package + attachments
4. Continue after per-item failures, but never map an item whose ASC gate
   failed; aggregate every error.
5. Re-read both systems and print a machine-readable summary plus a
   human-readable recap.

## Suggested natural-language prompts for MCP

- `List all apps in RevenueCat project PROJECT_ID and show which one matches bundle id com.example.app`
- `Create RevenueCat product com.example.premium.monthly as subscription in app APP_ID`
- `Create entitlement premium and attach product PRODUCT_ID`
- `Create offering default with packages $rc_monthly and $rc_annual`
- `Show complete offering configuration including packages and attached products`
