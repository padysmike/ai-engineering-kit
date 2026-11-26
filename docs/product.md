# Live Q&A Web App – Feature One-Pager

26\. 11. 2026

## Overview

A web app where an audience joins a live room by scanning a QR code, posts questions anonymously, and ranks the posts so the best ones rise to the top. No installs, no sign-up forms.

- Users: attendees (join, post, vote), hosts/moderators (create room, manage posts).
- Use cases: lectures, town halls, conferences, team meetings, workshops.
- Goal: lowest possible friction to join, and honest input because posts are anonymous.

## Core features

| Feature | What it does | Must-have rules |
|---|---|---|
| 1. QR login | Host's screen shows a QR code; scanning it opens the room in the phone browser and signs the attendee in automatically. | Code encodes a room URL with a short-lived token; no password or email; works on any mobile browser; manual fallback via a 6-character room code. |
| 2. Rooms | A room is one live session with its own feed, participants and settings. Host creates it, shares the QR, opens/closes it. | Unique room code; states: draft, open, closed (read-only); live participant count; host can rotate the QR or kick a participant. |
| 3. Anonymous posts | Attendees submit a question or comment without their identity shown to anyone, including the host. | Random per-room alias (e.g. "Blue Fox"), stable within the room; max 280 characters; rate limit per device; profanity filter; host can hide or delete posts. |
| 4. Ranking | Attendees upvote posts; the feed sorts by votes so top questions surface. | One vote per participant per post, toggleable; sort by top / newest; live updates without refresh; host can pin or mark as answered. |

## Key user flow

1. Host creates a room and displays its QR code on a screen.
2. Attendee scans the QR, lands in the room, and is signed in with an anonymous alias.
3. Attendee posts a question; it appears in the live feed for everyone.
4. Attendees upvote; the feed re-ranks in real time.
5. Host pins, hides or marks questions as answered, then closes the room.

Privacy rule: the server stores a session token per device for rate limiting and vote integrity, but never links it to a visible name. The host sees aliases only.

## Non-functional, scope and open questions

- Performance: feed updates reach all participants within 1 s; target 500 concurrent users per room.
- Platform: responsive web, mobile-first; no native app.
- Security: signed, expiring QR tokens; per-device rate limits; moderation tools for the host.
- Out of scope (v1): accounts and profiles, replies/threads, file uploads, analytics export, multi-host rooms.

### Open questions

- Can attendees post multiple questions, or is there a cap per person?
- Should votes be visible as counts only, or also show who voted (no, by default)?
- Should rooms auto-close after a set time, and is history kept afterwards?
