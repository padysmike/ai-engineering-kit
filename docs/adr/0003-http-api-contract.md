# 0003. HTTP JSON API with Server-Sent Events for the PoC and MVP

- Status: accepted
- Date: 2026-11-26

## Context
The PoC and MVP are built as 12 small slices, one feature file each (listed under "Slices"; QAAT-2 to QAAT-5 and QAAT-8 to QAAT-15). Their endpoints, and the web UI (QAAT-16), must agree before more of them exist. Some endpoints are already merged, and approved tests use them.

Constraints:
- Attendees are anonymous: others, including the host, see only an alias.
- The PoC has no auth: no accounts and no login.
- Changes must reach viewers without a reload. The product target is 1 s with 500 concurrent users per room.
- The standard library is preferred over new dependencies.

Items marked `Qn` depend on an undecided open question, listed at the end. Where this document gives such an item a behaviour, it is provisional.

## Decision
The API is JSON over HTTP. Two opaque bearer tokens identify the caller, and changes are pushed with Server-Sent Events (SSE).

### Existing endpoints
This is the merged behaviour. Paths and field names do not change.

| Request | Success | Errors |
|---|---|---|
| `POST /rooms` | `201` room, in state `draft` | |
| `GET /rooms/{code}` | `200` room | `404` |
| `POST /rooms/{code}/open` (host token) | `200` room; `draft` to `open` | `404`; `401`, `403`; `409` from any other state |
| `POST /rooms/{code}/close` (host token) | `200` room; `open` to `closed` | `404`; `401`, `403`; `409` from any other state |
| `POST /rooms/{code}/participants` | `201` `{"alias":"Blue Fox"}`; the request body may be empty or carry `qrToken` (see "QR code") | `404`; `403`, `400` for a bad body; `409` unless the room is `open` |
| `/healthz` (any method) | `200` `{"status":"ok"}` | |
| any other path | static files from `app/web` | `404` |

- A room is `{"code":"GEA33N","state":"open","participantCount":1}`.
- A code is 6 characters from `A`-`Z` and `2`-`7`, matched case-sensitively (Q8).
- An error body is plain text (`text/plain; charset=utf-8`), for example `room not found`.
- A wrong method on a `/rooms` path falls through to the static files, so it gets `404`, not `405`.
- A closed room cannot be reopened (Q9), and only an open room can be joined (draft: Q9). A closed room takes no new joins.

### Tokens
| Token | Issued by | Identifies |
|---|---|---|
| host token | `POST /rooms`, new field `hostToken` | the host of that one room |
| session token | `POST /rooms/{code}/participants`, new field `token` | one participant of that one room |

- Both are random, unguessable strings, sent as `Authorization: Bearer <token>`. They never go in a URL.
- `hostToken` appears only in the `201` response of `POST /rooms`, never in another room response.
- The server links a session token to a participant id and an alias, and to nothing that names a person.
- Join also returns `id`, a number that identifies the participant within the room: `{"id":1,"alias":"Blue Fox","token":"..."}`.
- The alias stays the same for one session token. A new join creates a new participant, so a client keeps its token to keep its alias (Q13). The API addresses participants by `id`.
- Aliases are unique within a room. An alias is a colour and an animal, from a list of 40 colours and 40 animals: 1,600 aliases. An alias stays used and is never freed, also after a kick. When none is left, join returns `409` with the text `no aliases left`. The other `409` on join, for a room that is not open, keeps its merged text `room is not open`. The ticket "Unique aliases within a room" (QAAT-20) adds uniqueness and the new response.

**When.** Slice 3 adds `hostToken`, `token` and `id`. These response fields are additive. Slice 3 uses the host token where the host needs its own view, such as reading the posts. Host-only endpoints added by later slices require it from the start.

**Change to existing behaviour:** `open` and `close` needed no token during the PoC. Since "Enforce host token on host actions" (QAAT-18), at the start of the MVP, they require the host token. A missing or unknown token, or one for another room, is `401`. A participant token of the room is `403`. This is not backwards compatible: a request without the token no longer works.

### Endpoints to add
"Host" and "participant" name the token required; that includes the host-only reads `/qr`, `/qr.png` and `/participants`. All paths in this table start with `/rooms/{code}`.

| Request | Token | Success | Slice |
|---|---|---|---|
| `POST /open`, `POST /close` (existing) | host | as above | 1; token enforced before 8 |
| `POST /posts` with `{"text":"..."}` | participant | `201` post | 3, 4 |
| `GET /posts?sort=top\|newest` | participant or host | `200` `{"posts":[post]}` | 3, 6 |
| `POST /posts/{postId}/vote` | participant | `200` post; adds the caller's vote, or removes it if present | 5, 7 |
| `GET /qr` | host | `200` `{"url":"..."}` | 8 |
| `GET /qr.png` | host | `200` a PNG image of the QR code | 8 |
| `POST /qr/rotate` | host | `200` `{"url":"..."}`, different from before | 9 |
| `GET /events` | participant or host | `200` event stream | 10 |
| `POST /posts/{postId}/hide` | host | `200` post | 11 |
| `DELETE /posts/{postId}` | host | `204` | 11 |
| `DELETE /participants/{participantId}` | host | `204` | 11 |
| `GET /participants` | host | `200` `{"participants":[{"id":1,"alias":"Blue Fox"}]}` | 11 (Q11) |
| `POST /posts/{postId}/pin` | host | `200` post | 12 |
| `POST /posts/{postId}/answered` | host | `200` post | 12 |

**Posts.** A post is `{"id":7,"alias":"Blue Fox","text":"...","votes":2}`. Slice 11 adds `hidden`, in host responses only. Slice 12 adds `pinned` and `answered`. `id` is a number that increases with each new post in the room.
- Text may be at most 280 characters, counted as Unicode code points (Q14). Whether empty text is accepted is undecided (Q14).
- A request body for a new post may be at most 4096 bytes, which fits 280 characters in any JSON escaping. A larger body is `400`.
- A closed room rejects new posts, new votes and vote removal. Its feed stays readable to earlier participants.
- Both orders put pinned posts first. Within the pinned posts, and within the others, the normal order applies: `sort=newest` is descending `id`; `sort=top` is descending `votes`, with ties oldest first (ascending `id`). Without `sort`, the order is `newest`.
- Any other value of `sort`, including an empty one, is `400`. Pinning comes with slice 12.
- Marking as answered does not change the order (Q18). Neither mark can be undone (Q18).
- Whether participants may vote for their own posts is undecided (Q16).
- Votes are exposed as a count only, never as who voted (Q2).

**What each side sees.**

| | Participant | Host |
|---|---|---|
| Authors | alias | alias |
| Hidden posts | left out; voting on one is `404` | included, with `"hidden":true` |
| Deleted posts | gone | gone |
| Participant list | no | `id` and alias |

No response or event contains a session token, apart from the join response that issues it.

**Kick.** `DELETE /participants/{participantId}` revokes that session token: later requests with it get `401`, its event stream is closed, and `participantCount` drops by one. The person can join again with the room code (Q11). The count does not drop when someone only disconnects (Q12). What happens to a kicked participant's posts and votes is not specified; they stay (Q11).

### QR code
- `url` points to the app's join page for that room and carries the room code and a signed, expiring QR token. Clients treat the URL as opaque. The token lifetime is undecided (Q6).
- Opening the URL loads the page and changes nothing. The page then sends `POST /rooms/{code}/participants` with `{"qrToken":"..."}`.
- If a join carries `qrToken`, the server accepts it only if it is valid for that room and not expired. Otherwise it answers `403` and adds no participant. The page does not retry without the token.
- A join without `qrToken` remains the manual room-code join.
- Rotating makes earlier QR tokens invalid. It does not change the room code or sign anyone out (Q6, Q7).
- `GET /qr.png` draws the QR code of the URL that `GET /qr` would return at that moment. The host's page fetches it with the host token and shows it from a `blob:` URL, so its content security policy allows `img-src 'self' blob:`. Both responses carry `Cache-Control: no-store`.
- The Go standard library has no QR encoder. The server uses `rsc.io/qr` v0.2.0 (BSD-3-Clause), approved by Michal on 2026-11-26.
- The QR token is signed with HMAC-SHA256 under a random key of the room. It covers the room code, a generation number for later rotation, and the expiry time.
- The URL is `<scheme>://<host>/#room=<code>&qr=<token>`, with the scheme and host of the request that asked for it. The token is in the fragment, which a browser does not send to the server. A host who opens the app as `localhost` therefore gets a QR code that other devices cannot use.
- A QR token lasts 5 minutes (provisional, Q6). The host's screen fetches a new one every minute.
- The join body may be empty or JSON, at most 4096 bytes. Anything else is `400`.

### Live updates
`GET /rooms/{code}/events` answers with `text/event-stream`. Each `data` line is JSON.

| Event | Data | Sent |
|---|---|---|
| `snapshot` | `{"room":room,"posts":[post]}` | first, on every connection |
| `post` | post | a post is created or changed; replaces any post with the same `id` |
| `post-removed` | `{"id":7}` | a post is deleted, or hidden (to participants) |
| `room` | room | the state or participant count changes |

- Events follow the same visibility rules as the feed.
- The server registers the subscriber and takes the snapshot atomically, so no change falls between them. A client reconnects after any disconnect and replaces its state with the new snapshot. There is no event replay.
- The client applies the sort rules above to its own copy, so an event does not depend on a viewer's sort order.
- Publishing never waits for a subscriber. The server closes a stream that falls too far behind (256 waiting events, or one write that takes over 10 s), and the client reconnects.
- The server sends a comment line every 15 s to keep idle streams open. It must not set an HTTP write timeout that cuts streams off.
- The browser reads the stream with `fetch`, because `EventSource` cannot send an `Authorization` header. The client therefore does its own reconnecting.

### Status codes
New endpoints keep plain-text error bodies.

| Status | Meaning |
|---|---|
| `200`, `201`, `204` | success; created; success without a body |
| `400` | malformed JSON, text over 280 characters, or unknown `sort` |
| `401` | token missing, unknown, revoked, or for another room |
| `403` | known token of the wrong kind for this request, or invalid or expired `qrToken` |
| `404` | unknown room, post or participant |
| `409` | the room state does not allow the request, or a join finds no free alias |

When several apply, the server checks the room (`404`), then the token (`401`, `403`), then the post or participant (`404`), then the room state (`409`), then the input (`400`).

### Slices
| Slice | Feature file |
|---|---|
| 1, 2 | `room-lifecycle`, `join-by-code` (merged) |
| 3, 4 | `posting`, `post-limits`; slice 3 also issues both tokens |
| after 4 | `alias-uniqueness`: "Unique aliases within a room" |
| 5, 6, 7 | `voting`, `feed-sorting`, `closed-room-voting` |
| before 8 | no feature file: "Enforce host token on host actions" |
| 8, 9 | `qr-login`, `qr-tokens` |
| 10, 11, 12 | `live-updates`, `moderation`, `post-marks` |

Slices 1 to 7, with the alias ticket, are the PoC. Slices 11 and 12 also send the `post`, `post-removed` and `room` events for their changes.

### Rejected alternatives
- **WebSocket.** Only the server needs to push; every change a client makes is an ordinary request. The Go standard library has no WebSocket server, so it would add a dependency. SSE needs only `net/http` and `http.Flusher`.
- **Polling.** Meeting 1 s means each of 500 clients asks at least once a second, mostly to learn that nothing changed.
- **Events that only say "reload the feed", or send the whole feed each time.** Either way every change makes the server send the full feed to all 500 clients.
- **Token in the query string or a cookie for the stream.** A URL ends up in logs and history. A cookie would be a second way to pass tokens and would need protection against cross-site requests.
- **Joining on `GET` of the QR URL.** Link previews, prefetching and page reloads would create participants.
- **JSON error bodies.** They would change the existing responses, and no scenario needs them.
- **Separate `PUT` and `DELETE` for votes.** They are safe to retry, but the scenarios describe one action: upvoting again removes the vote.

## Consequences
- Slices can be built separately against one contract, with no new dependency for the API or live updates.
- Two rules run in the browser, so scenario steps have to play the client and do not test the page's code: ordering the live feed (slice 10), and the join that follows opening the QR URL (slice 8). Both need a manual check.
- Knowing a room code is not enough to open or close that room. Losing the host token means losing control of the actions that require it.
- The tokens are not user authentication. Anyone can create rooms, and anyone who knows the code of an open room can join, any number of times. So a kick or an expired QR token does not keep a person out, and one person can vote more than once by joining again.
- Every join takes one of the 1,600 aliases and none is freed. That covers 500 participants with room for 1,100 more joins; after that nobody else can join the room. Since anyone with the code can join repeatedly, one person can use them all up.
- The per-device rate limit (Q4) and profanity filter (Q5) are not in this contract.
- Rooms, tokens and subscribers live in the memory of one process. A restart loses them, and running several instances would need a different design (Q3).
- 500 users within 1 s is not verified (Q19). A change in a room of 500 means about 500 small writes, but many votes at once multiply that, and a reconnect of everyone sends 500 snapshots. A load test has to confirm it.
- Over HTTP/1.1 a browser limits connections to one host, so several tabs with open streams can stall one another. HTTP/2 avoids this.
- Proxies in front of the app must not buffer the event stream.

## Open questions
| Q | Question | Affects |
|---|---|---|
| Q2 | Are votes shown only as counts, or also who voted (default: no)? | post |
| Q3 | Do rooms close automatically, and is history kept? | storage; no endpoint |
| Q4 | What are the per-device rate limits? | not in this contract |
| Q5 | What does the profanity filter catch and do? | not in this contract |
| Q6 | How long does a QR token last, and does expiry affect people already signed in? | QR |
| Q7 | After rotation, do old QR codes stop working at once, and does the room code change? | QR |
| Q8 | Room code characters, case sensitivity, unknown codes, reuse after closing? | codes |
| Q9 | Can people join or post in a draft room, and can a closed room be reopened? | room states |
| Q10 | Can the host still moderate a closed room? | moderation. Provisional: host actions on posts and participants still work |
| Q11 | Can a kicked participant rejoin, and how does the host pick one? | participants, kick |
| Q12 | Does the live count drop on disconnect? | `participantCount` |
| Q13 | Does rejoining on a device keep the alias? | alias |
| Q14 | How are the 280 characters counted, and are empty posts allowed? | posts |
| Q15 | Can a hidden post be restored, and can authors delete posts? | moderation. No restore or author delete here |
| Q16 | Can participants upvote their own posts? | votes |
| Q18 | Can several posts be pinned, can marks be removed, and does answered affect the order? | marks. Provisional: any number can be pinned |
| Q19 | How is the 1 s / 500-user target tested? | live updates |

Q1 (a cap on posts per person) adds nothing to this contract until decided.
