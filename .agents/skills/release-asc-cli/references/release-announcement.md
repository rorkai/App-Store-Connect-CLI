# Release announcement

Every release gets a short film and a thread. The thread reuses the same posts on X, Threads, Bluesky, and Mastodon; LinkedIn stays off.

## Render the release film

The films live in the private `rudrankriyam/asc-motion` repository (locally `~/Developer/CLIs/asc-motion`, Remotion with bun). If that checkout is unavailable, skip the film, say so in the handoff, and post the thread without it.

1. From the repository root run `./new-release-film.sh NNNN-asc-X-Y X.Y.Z`, where `NNNN` is the next free folder number.
2. Edit only `NNNN-asc-X-Y/src/content.ts`. Take three or four headline features from the website changelog entry for the version, each with one exact command from the matching guide. Never invent a flag; check every command against `asc <command> --help`. Put the remaining notable changes in the `FIXES` list, one line each under 60 characters.
3. Inside the folder run `bun install && bun run render`. The music bed is synthesised on every render; nothing licensed is involved.
4. Render one still per beat with `bunx remotion still AscRelease --frame N out/<beat>.png` and look at each one. Captions must stay clear of the `>_` mark and commands must finish typing before the beat ends; shorten the caption or command if not.
5. Commit the folder on a branch named `release-film-X-Y` in `asc-motion` and open a pull request there. Merge only when the user asks.

Do not reuse the Rork motion kit grammar (cursor, gradient pill, camera moves, click sounds, grain). The film is typographic: version, product name, orange rule, typed commands.

## Thread format

Post 1 carries the film and nothing else that the film already types:

```
App Store Connect CLI X.Y.Z

<one line naming the three or four headline features>

https://github.com/rorkai/App-Store-Connect-CLI
```

No sign-off line after the feature line, no emojis, no teaser such as "every change is in the thread below".

Then one post per change in the changelog, with its command inline as plain text (no backticks). Keep every post under Bluesky's 300-character limit so the thread is identical on all four platforms.

The last post is only:

```
Full changelog:

https://asccli.sh/changelog
```

## Typefully workflow

Use this section only when an external draft is authorized. Otherwise include the finished posts in the local handoff without creating a remote draft.

1. Request a media upload for the rendered `.mp4` with alt text that names the version and the beats. Upload the file to the returned presigned URL with `curl -T <file> "<upload_url>"` and no `Content-Type` header; adding one invalidates the signature. Poll the media status until it is `ready`.
2. If an older text-only draft for the same version exists, rewrite it in place with the thread above instead of creating a second draft; delete any leftover duplicate. Published drafts cannot be edited through the API, so leave those alone.
3. Resolve the user's social set, enable X, Threads, Bluesky, and Mastodon, disable LinkedIn, attach the media ID to post 1 on every enabled platform, and save as an unscheduled draft only.
4. Verify the returned state is `draft` and return its review URL.

If the Typefully connector is unavailable, preserve the finished posts in the handoff and report the external draft as incomplete, separately from artifact and distribution verification. Never silently publish through another tool.
