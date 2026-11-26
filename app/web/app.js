"use strict";

const SESSION_KEY = "qa-session";
const MAX_POST_CHARACTERS = 280;
const FIRST_RETRY_MILLIS = 1000;
const LONGEST_RETRY_MILLIS = 10000;
// Three missed server heartbeats mean the connection is dead.
const SILENCE_LIMIT_MILLIS = 45000;
const PERMANENT_STATUSES = [401, 403, 404];
const QR_REFRESH_MILLIS = 60_000;

const byId = (id) => document.getElementById(id);

let session = loadSession();
let busy = false;
let posts = new Map();
let stream = null;
let live = false;
let streamedChanges = 0;
let qrWanted = false;
let qrRequests = 0;
let participantsStale = false;
let loadingParticipants = false;

function loadSession() {
  try {
    const stored = JSON.parse(sessionStorage.getItem(SESSION_KEY));
    if (stored === null) return null;
    if (stored.code && stored.token) return { votedPostIds: [], ...stored };
  } catch {
    // An unreadable value is discarded like an incomplete one.
  }
  sessionStorage.removeItem(SESSION_KEY);
  return null;
}

async function send(method, path, { token, body } = {}) {
  const headers = {};
  if (token) headers.Authorization = `Bearer ${token}`;
  if (body) headers["Content-Type"] = "application/json";
  let response;
  try {
    response = await fetch(path, { method, headers, body: body && JSON.stringify(body), cache: "no-store" });
  } catch {
    throw new Error("Could not reach the server");
  }
  if (!response.ok) {
    throw new Error((await response.text()).trim() || `Request failed (${response.status})`);
  }
  return response;
}

async function api(method, path, options) {
  const response = await send(method, path, options);
  return response.status === 204 ? null : response.json();
}

const roomPath = () => `/rooms/${encodeURIComponent(session.code)}`;

function showError(message) {
  byId("error").textContent = message;
  byId("error").hidden = !message;
}

// One request chain at a time, so a double tap cannot toggle a vote twice.
async function perform(action) {
  if (busy) return;
  busy = true;
  document.body.setAttribute("aria-busy", "true");
  showError("");
  try {
    await action();
  } catch (error) {
    showError(error.message);
  } finally {
    busy = false;
    document.body.removeAttribute("aria-busy");
  }
}

function saveSession() {
  sessionStorage.setItem(SESSION_KEY, JSON.stringify(session));
}

function showScreen() {
  byId("home").hidden = Boolean(session);
  byId("room").hidden = !session;
  if (!session) return;
  const isHost = session.role === "host";
  byId("host-controls").hidden = !isHost;
  byId("participants").hidden = !isHost;
  byId("post-form").hidden = isHost;
  byId("room-code").textContent = session.code;
  byId("alias").textContent = session.alias ?? "";
}

function clearRoom() {
  byId("room-state").textContent = "";
  byId("participant-count").textContent = "";
  byId("open-room").disabled = true;
  byId("close-room").disabled = true;
  byId("rotate-qr").disabled = true;
  posts = new Map();
  qrWanted = false;
  replaceQRImage(null);
  byId("feed").replaceChildren();
  byId("participant-list").replaceChildren();
  byId("empty-feed").hidden = true;
  byId("post-form").reset();
  byId("sort").value = "newest";
  showPostLength();
}

function showRoom(room) {
  byId("room-state").textContent = room.state;
  byId("participant-count").textContent = room.participantCount;
  byId("open-room").disabled = room.state !== "draft";
  byId("close-room").disabled = room.state !== "open";
  byId("rotate-qr").disabled = room.state !== "open";
  syncQR(room);
  if (session.role === "host") updateParticipants();
}

function showParticipants(list) {
  byId("participant-list").replaceChildren(
    ...list.map(({ id, alias }) => {
      const item = document.createElement("li");
      item.className = "row spread";
      item.append(
        textElement("span", "", alias),
        actionButton("Kick", `Kick ${alias}? Their posts and votes stay.`, () => kick(id)),
      );
      return item;
    }),
  );
}

// Many joins arrive at once, so changes during a request cause just one more.
async function updateParticipants() {
  participantsStale = true;
  if (loadingParticipants) return;
  loadingParticipants = true;
  try {
    while (participantsStale && session?.role === "host") {
      participantsStale = false;
      const asking = session;
      const { participants } = await api("GET", `${roomPath()}/participants`, { token: asking.token });
      if (session === asking) showParticipants(participants);
    }
  } catch (error) {
    showError(error.message);
  } finally {
    loadingParticipants = false;
  }
}

function actionButton(label, question, action) {
  const button = textElement("button", "secondary", label);
  button.type = "button";
  button.addEventListener("click", () => {
    if (!question || confirm(question)) perform(action);
  });
  return button;
}

function textElement(tag, className, text) {
  const element = document.createElement(tag);
  element.className = className;
  element.textContent = text;
  return element;
}

function voteButton(post) {
  const voted = session.votedPostIds.includes(post.id);
  const button = textElement("button", "", voted ? "Remove vote" : "Upvote");
  button.type = "button";
  button.setAttribute("aria-pressed", String(voted));
  button.addEventListener("click", () => perform(() => toggleVote(post.id)));
  return button;
}

function postItem(post) {
  const votes = document.createElement("div");
  votes.className = "row spread votes";
  votes.append(textElement("span", "", `${post.votes} ${post.votes === 1 ? "vote" : "votes"}`));
  if (session.role === "attendee") votes.append(voteButton(post));
  const item = document.createElement("li");
  item.append(textElement("p", "author", post.alias));
  if (post.pinned) item.append(textElement("span", "mark", "Pinned"));
  if (post.answered) item.append(textElement("span", "mark", "Answered"));
  item.append(textElement("p", "text", post.text), votes);
  if (session.role === "host") item.append(moderationControls(post));
  return item;
}

function moderationControls(post) {
  const controls = document.createElement("div");
  controls.className = "row";
  if (!post.pinned) {
    controls.append(actionButton("Pin", "Pin this post? This cannot be undone.", () => pinPost(post.id)));
  }
  if (!post.answered) {
    const question = "Mark this post as answered? This cannot be undone.";
    controls.append(actionButton("Mark answered", question, () => markAnswered(post.id)));
  }
  controls.append(
    post.hidden
      ? textElement("span", "hidden-mark", "Hidden from attendees")
      : actionButton("Hide", "", () => hidePost(post.id)),
    actionButton("Delete", "Delete this post for everyone?", () => deletePost(post.id)),
  );
  return controls;
}

const pinnedFirst = (a, b) => Number(b.pinned) - Number(a.pinned);
const newestFirst = (a, b) => pinnedFirst(a, b) || b.id - a.id;
const mostVotedFirst = (a, b) => pinnedFirst(a, b) || b.votes - a.votes || a.id - b.id;

// Sorting happens here, so a new sort order needs no request.
function showFeed() {
  const order = byId("sort").value === "top" ? mostVotedFirst : newestFirst;
  byId("feed").replaceChildren(...[...posts.values()].sort(order).map(postItem));
  byId("empty-feed").hidden = posts.size > 0;
}

function replaceFeed(list) {
  posts = new Map(list.map((post) => [post.id, post]));
  showFeed();
}

function replaceQRImage(url) {
  const previous = byId("qr").getAttribute("src");
  if (previous) URL.revokeObjectURL(previous);
  if (url) byId("qr").setAttribute("src", url);
  else byId("qr").removeAttribute("src");
  byId("qr").hidden = !url;
}

// An image request cannot carry the host token, so the page fetches the image.
// A QR token expires, so this also runs to replace the code on screen.
async function renewQR() {
  if (!qrWanted || !session) return;
  const { code, token } = session;
  const request = ++qrRequests;
  try {
    const image = await send("GET", `${roomPath()}/qr.png`, { token });
    const blob = await image.blob();
    // A slower, older request must not bring back a code that was rotated away.
    if (request === qrRequests && qrWanted && session?.code === code) replaceQRImage(URL.createObjectURL(blob));
  } catch (error) {
    showError(error.message);
  }
}

// Room updates arrive often, so the image is fetched only when it appears.
function syncQR(room) {
  const wanted = session.role === "host" && room.state === "open";
  if (wanted === qrWanted) return;
  qrWanted = wanted;
  if (wanted) renewQR();
  else replaceQRImage(null);
}

async function refresh() {
  const changesBefore = streamedChanges;
  const room = await api("GET", roomPath());
  const feed = await api("GET", `${roomPath()}/posts`, { token: session.token });
  if (streamedChanges !== changesBefore) return;
  showRoom(room);
  replaceFeed(feed.posts);
}

async function refreshUnlessLive() {
  if (!live) await refresh();
}

const eventHandlers = {
  snapshot: ({ room, posts: list }) => {
    showRoom(room);
    replaceFeed(list);
  },
  room: showRoom,
  post: (post) => {
    posts.set(post.id, post);
    showFeed();
  },
  "post-removed": ({ id }) => {
    posts.delete(id);
    showFeed();
  },
};

function applyEvent(block) {
  const lines = block.split("\n");
  const field = (name) => lines.find((line) => line.startsWith(`${name}: `))?.slice(name.length + 2);
  const handler = eventHandlers[field("event")];
  if (!handler) return false;
  streamedChanges += 1;
  handler(JSON.parse(field("data")));
  return true;
}

function showLiveState(isLive) {
  live = isLive;
  byId("live-state").textContent = isLive ? "Live updates on" : "Not live. Reconnecting; Reload also works.";
}

async function readEvents(connection, onSnapshot) {
  let silence;
  const expectData = () => {
    clearTimeout(silence);
    silence = setTimeout(() => connection.abort(), SILENCE_LIMIT_MILLIS);
  };
  try {
    expectData();
    const response = await fetch(`${roomPath()}/events`, {
      headers: { Authorization: `Bearer ${session.token}` },
      cache: "no-store",
      signal: connection.signal,
    });
    if (!response.ok) {
      const error = new Error((await response.text()).trim() || `Request failed (${response.status})`);
      error.status = response.status;
      error.permanent = PERMANENT_STATUSES.includes(response.status);
      throw error;
    }
    const reader = response.body.pipeThrough(new TextDecoderStream()).getReader();
    let pending = "";
    for (;;) {
      const { value, done } = await reader.read();
      if (done || connection.signal.aborted) return;
      expectData();
      const blocks = (pending + value).split("\n\n");
      pending = blocks.pop();
      for (const block of blocks) {
        if (applyEvent(block) && !live) {
          showLiveState(true);
          onSnapshot();
        }
      }
    }
  } finally {
    clearTimeout(silence);
  }
}

// EventSource cannot send the Authorization header, so reconnecting is ours.
// Every connection starts with a snapshot, which replaces what was shown.
async function followRoom(following) {
  let retryMillis = FIRST_RETRY_MILLIS;
  while (stream === following) {
    following.connection = new AbortController();
    try {
      await readEvents(following.connection, () => (retryMillis = FIRST_RETRY_MILLIS));
    } catch (error) {
      if (stream !== following) return;
      if (error.permanent) {
        live = false;
        byId("live-state").textContent = "Not live.";
        if (error.status === 401) replaceFeed([]);
        showError(error.message);
        return;
      }
    }
    if (stream !== following) return;
    showLiveState(false);
    await new Promise((resolve) => setTimeout(resolve, retryMillis));
    retryMillis = Math.min(retryMillis * 2, LONGEST_RETRY_MILLIS);
  }
}

function startStream() {
  stopStream();
  stream = {};
  showLiveState(false);
  followRoom(stream);
}

function stopStream() {
  stream?.connection?.abort();
  stream = null;
  live = false;
}

async function reload() {
  stopStream();
  try {
    await refresh();
  } finally {
    startStream();
  }
}

async function enterRoom(next) {
  session = next;
  saveSession();
  clearRoom();
  showScreen();
  startStream();
}

async function createRoom() {
  const room = await api("POST", "/rooms");
  await enterRoom({ role: "host", code: room.code, token: room.hostToken });
}

async function joinRoom(code, body) {
  const joined = await api("POST", `/rooms/${encodeURIComponent(code)}/participants`, { body });
  byId("join-form").reset();
  await enterRoom({ role: "attendee", code, token: joined.token, alias: joined.alias, votedPostIds: [] });
}

const joinTypedRoom = () => joinRoom(byId("join-code").value.toUpperCase());

// A scanned QR URL carries the room and its token in the fragment. The
// fragment is removed, so that a reload does not join again.
function takeScannedQR() {
  const fragment = new URLSearchParams(location.hash.slice(1));
  const code = fragment.get("room");
  const qrToken = fragment.get("qr");
  if (!code || !qrToken) return null;
  history.replaceState(null, "", location.pathname + location.search);
  return { code, qrToken };
}

function joinScannedRoom() {
  const scanned = takeScannedQR();
  if (!scanned || session?.code === scanned.code) return;
  perform(() => joinRoom(scanned.code, { qrToken: scanned.qrToken }));
}

async function changeRoom(change) {
  await api("POST", `${roomPath()}/${change}`, { token: session.token });
  await refreshUnlessLive();
}

async function rotateQR() {
  await api("POST", `${roomPath()}/qr/rotate`, { token: session.token });
  replaceQRImage(null);
  await renewQR();
}

async function moderate(method, path) {
  await api(method, `${roomPath()}${path}`, { token: session.token });
  await refreshUnlessLive();
}

const pinPost = (postId) => moderate("POST", `/posts/${postId}/pin`);
const markAnswered = (postId) => moderate("POST", `/posts/${postId}/answered`);
const hidePost = (postId) => moderate("POST", `/posts/${postId}/hide`);
const deletePost = (postId) => moderate("DELETE", `/posts/${postId}`);
const kick = (participantId) => moderate("DELETE", `/participants/${participantId}`);

async function submitPost() {
  await api("POST", `${roomPath()}/posts`, { token: session.token, body: { text: byId("post-text").value } });
  byId("post-form").reset();
  showPostLength();
  await refreshUnlessLive();
}

// The API does not say whether the caller has voted, so the tab remembers it.
async function toggleVote(postId) {
  await api("POST", `${roomPath()}/posts/${postId}/vote`, { token: session.token });
  const voted = session.votedPostIds;
  session.votedPostIds = voted.includes(postId) ? voted.filter((id) => id !== postId) : [...voted, postId];
  saveSession();
  showFeed();
  await refreshUnlessLive();
}

function showPostLength() {
  const length = [...byId("post-text").value].length;
  byId("post-length").textContent = `${length} / ${MAX_POST_CHARACTERS} characters`;
  byId("post-length").classList.toggle("over", length > MAX_POST_CHARACTERS);
}

function leaveRoom() {
  if (busy || !confirm("Leave this room? This tab cannot get back in as the same host or alias.")) return;
  stopStream();
  session = null;
  sessionStorage.removeItem(SESSION_KEY);
  showError("");
  showScreen();
}

function onSubmit(formId, action) {
  byId(formId).addEventListener("submit", (event) => {
    event.preventDefault();
    perform(action);
  });
}

onSubmit("join-form", joinTypedRoom);
onSubmit("post-form", submitPost);
byId("create-room").addEventListener("click", () => perform(createRoom));
byId("open-room").addEventListener("click", () => perform(() => changeRoom("open")));
byId("close-room").addEventListener("click", () => perform(() => changeRoom("close")));
byId("rotate-qr").addEventListener("click", () => perform(rotateQR));
byId("reload").addEventListener("click", () => perform(reload));
byId("sort").addEventListener("change", showFeed);
byId("post-text").addEventListener("input", showPostLength);
byId("leave").addEventListener("click", leaveRoom);

window.addEventListener("hashchange", joinScannedRoom);
setInterval(renewQR, QR_REFRESH_MILLIS);

clearRoom();
showScreen();
if (session) startStream();
joinScannedRoom();
