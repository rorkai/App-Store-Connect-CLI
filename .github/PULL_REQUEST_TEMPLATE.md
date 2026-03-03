## Summary

- 

## Validation

- [ ] `make format`
- [ ] `make lint`
- [ ] `ASC_BYPASS_KEYCHAIN=1 make test`

## Wall of Apps (only if this PR adds/updates a Wall app)

- [ ] I ran `make generate app APP="..." LINK="..." CREATOR="..." PLATFORM="..."` (or manually edited `docs/wall-of-apps.json` + ran `make update-wall-of-apps`)
- [ ] I committed all generated files:
  - `docs/wall-of-apps.json`
  - `README.md`

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
