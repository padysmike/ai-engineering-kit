"use strict";

const SESSION_KEY = "qa-session";
const MAX_POST_CHARACTERS = 280;
const FIRST_RETRY_MILLIS = 1000;
const LONGEST_RETRY_MILLIS = 10000;
// Three missed server heartbeats mean the connection is dead.
const SILENCE_LIMIT_MILLIS = 45000;
const PERMANENT_STATUSES = [401, 403, 404];

const byId = (id) => document.getElementById(id);

let session = loadSession();
let busy = false;
let posts = new Map();
let stream = null;
let live = false;
let streamedChanges = 0;

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

async function api(method, path, { token, body } = {}) {
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
  return response.json();
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
  byId("post-form").hidden = isHost;
  byId("room-code").textContent = session.code;
  byId("alias").textContent = session.alias ?? "";
}

function clearRoom() {
  byId("room-state").textContent = "";
  byId("participant-count").textContent = "";
  byId("open-room").disabled = true;
  byId("close-room").disabled = true;
  posts = new Map();
  byId("feed").replaceChildren();
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
  votes.className = "row votes";
  votes.append(textElement("span", "", `${post.votes} ${post.votes === 1 ? "vote" : "votes"}`));
  if (session.role === "attendee") votes.append(voteButton(post));
  const item = document.createElement("li");
  item.append(textElement("p", "author", post.alias), textElement("p", "text", post.text), votes);
  return item;
}

const newestFirst = (a, b) => b.id - a.id;
const mostVotedFirst = (a, b) => b.votes - a.votes || a.id - b.id;

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

async function joinRoom() {
  const code = byId("join-code").value.toUpperCase();
  const joined = await api("POST", `/rooms/${encodeURIComponent(code)}/participants`);
  byId("join-form").reset();
  await enterRoom({ role: "attendee", code, token: joined.token, alias: joined.alias, votedPostIds: [] });
}

async function changeRoom(change) {
  await api("POST", `${roomPath()}/${change}`, { token: session.token });
  await refreshUnlessLive();
}

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

onSubmit("join-form", joinRoom);
onSubmit("post-form", submitPost);
byId("create-room").addEventListener("click", () => perform(createRoom));
byId("open-room").addEventListener("click", () => perform(() => changeRoom("open")));
byId("close-room").addEventListener("click", () => perform(() => changeRoom("close")));
byId("reload").addEventListener("click", () => perform(reload));
byId("sort").addEventListener("change", showFeed);
byId("post-text").addEventListener("input", showPostLength);
byId("leave").addEventListener("click", leaveRoom);

clearRoom();
showScreen();
if (session) startStream();
