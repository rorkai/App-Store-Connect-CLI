## Summary

- 

## Validation

- [ ] `make format`
- [ ] `make lint`
- [ ] `ASC_BYPASS_KEYCHAIN=1 make test`

## Wall of Apps (only if this PR adds/updates a Wall app)

- [ ] I used `asc apps wall submit --app "1234567890" --confirm` (or made the equivalent single-file update manually)
- [ ] This PR only updates `docs/wall-of-apps.json`

Entry template:

```json
{
  "app": "Your App Name",
  "link": "https://apps.apple.com/app/id1234567890",
  "creator": "your-github-handle",
  "platform": ["iOS"]
}
```

Common Apple labels: `iOS`, `macOS`, `watchOS`, `tvOS`, `visionOS`.
